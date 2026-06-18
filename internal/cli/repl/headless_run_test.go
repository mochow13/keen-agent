package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/session"
	"github.com/mochow13/keen-agent/internal/tools"
)

type recordingHeadlessClient struct {
	events   []llm.StreamEvent
	messages [][]llm.Message
	opts     [][]llm.StreamOptions
}

func (c *recordingHeadlessClient) StreamChat(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
	c.messages = append(c.messages, llm.CloneMessages(messages))
	c.opts = append(c.opts, append([]llm.StreamOptions(nil), opts...))
	ch := make(chan llm.StreamEvent, len(c.events))
	go func() {
		defer close(ch)
		for _, event := range c.events {
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (c *recordingHeadlessClient) Reset() {}

func TestRunHeadless_CreatesSessionAndWritesText(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "hello"},
		{Type: llm.StreamEventTypeUsage, Usage: &llm.TokenUsage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5}},
		{Type: llm.StreamEventTypeDone},
	}}
	var out bytes.Buffer

	result, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "say hi",
		Out:        &out,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}
	if result.SessionID == "" {
		t.Fatal("expected session id")
	}
	if result.OpenCodeSessionID == "" || result.OpenCodeSessionID == result.SessionID {
		t.Fatalf("expected hyphen-stripped OpenCode session id, got %q from %q", result.OpenCodeSessionID, result.SessionID)
	}
	if result.Text != "hello" || out.String() != "hello\n" {
		t.Fatalf("unexpected output result=%q out=%q", result.Text, out.String())
	}
	if result.Usage == nil || result.Usage.InputTokens != 3 || result.Usage.OutputTokens != 2 || result.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}

	events := loadOnlyHeadlessSessionEvents(t, workingDir)
	if len(events) != 3 {
		t.Fatalf("expected session started, user, assistant events; got %d", len(events))
	}
	if events[1].UserMessage == nil || events[1].UserMessage.Content != "say hi" {
		t.Fatalf("unexpected user event: %#v", events[1].UserMessage)
	}
	if events[2].AssistantTurn == nil || events[2].AssistantTurn.Message != "hello" {
		t.Fatalf("unexpected assistant event: %#v", events[2].AssistantTurn)
	}
}

func TestRunHeadless_ResumesSessionConversation(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	firstClient := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "first response"},
		{Type: llm.StreamEventTypeDone},
	}}

	first, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     firstClient,
		Prompt:     "first prompt",
	})
	if err != nil {
		t.Fatalf("first RunHeadless() error = %v", err)
	}

	secondClient := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "second response"},
		{Type: llm.StreamEventTypeDone},
	}}
	_, err = RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     secondClient,
		SessionID:  first.SessionID,
		Prompt:     "second prompt",
	})
	if err != nil {
		t.Fatalf("second RunHeadless() error = %v", err)
	}
	if len(secondClient.messages) != 1 {
		t.Fatalf("expected one StreamChat call, got %d", len(secondClient.messages))
	}
	got := messageContents(secondClient.messages[0])
	want := []string{"first prompt", "first response", "second prompt"}
	if !containsOrderedSuffix(got, want) {
		t.Fatalf("expected conversation suffix %#v, got %#v", want, got)
	}
	if len(secondClient.opts) != 1 || len(secondClient.opts[0]) != 1 || secondClient.opts[0][0].SessionID != first.SessionID {
		t.Fatalf("expected session stream option %q, got %#v", first.SessionID, secondClient.opts)
	}
}

func TestRunHeadless_WritesJSON(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "json response"},
		{Type: llm.StreamEventTypeDone},
	}}
	var out bytes.Buffer

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "prompt",
		Format:     HeadlessFormatJSON,
		Out:        &out,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}

	var decoded HeadlessRunResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	if decoded.SessionID == "" || decoded.Text != "json response" {
		t.Fatalf("unexpected json result: %#v", decoded)
	}
}

func setupHeadlessTestHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	workingDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		t.Fatalf("create working dir: %v", err)
	}
	return workingDir
}

func headlessTestConfig() *config.ResolvedConfig {
	return &config.ResolvedConfig{
		Provider: config.ProviderOpenAI,
		APIKey:   "test-key",
		Model:    "test-model",
	}
}

func loadOnlyHeadlessSessionEvents(t *testing.T, workingDir string) []session.Event {
	t.Helper()
	store, err := session.NewStore(workingDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	summaries, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one session, got %d", len(summaries))
	}
	loaded, err := store.Load(summaries[0])
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return loaded.Events
}

func messageContents(messages []llm.Message) []string {
	contents := make([]string, 0, len(messages))
	for _, message := range messages {
		contents = append(contents, message.Content)
	}
	return contents
}

func containsOrderedSuffix(got []string, want []string) bool {
	if len(got) < len(want) {
		return false
	}
	got = got[len(got)-len(want):]
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
