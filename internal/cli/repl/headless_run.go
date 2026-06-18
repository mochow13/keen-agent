package repl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	repltooling "github.com/mochow13/keen-agent/internal/cli/repl/tooling"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/session"
)

const (
	HeadlessFormatText = "text"
	HeadlessFormatJSON = "json"
)

type HeadlessRunOptions struct {
	WorkingDir string
	Config     *config.ResolvedConfig
	Client     llm.LLMClient
	SessionID  string
	Prompt     string
	Format     string
	Out        io.Writer
}

type HeadlessRunResult struct {
	SessionID         string         `json:"session_id"`
	OpenCodeSessionID string         `json:"opencode_session_id"`
	Text              string         `json:"text"`
	Usage             *headlessUsage `json:"usage,omitempty"`
}

type headlessUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
	TotalTokens     int `json:"total_tokens"`
	CachedTokens    int `json:"cached_tokens"`
}

func RunHeadless(ctx context.Context, opts HeadlessRunOptions) (*HeadlessRunResult, error) {
	prompt := strings.TrimSpace(opts.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if opts.Client == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}
	format := opts.Format
	if format == "" {
		format = HeadlessFormatText
	}
	if format != HeadlessFormatText && format != HeadlessFormatJSON {
		return nil, fmt.Errorf("unsupported format %q", format)
	}

	appState := replappstate.New(opts.Client, opts.WorkingDir)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := repltooling.NewDiffEmitter()
	repltooling.SetupToolRegistry(opts.WorkingDir, appState, permissionRequester, diffEmitter, nil, opts.Config)

	sessions := newReplSessionState(opts.WorkingDir)
	if sessions == nil {
		return nil, fmt.Errorf("session store unavailable")
	}
	if opts.SessionID != "" {
		loaded, err := loadHeadlessSession(sessions, opts.SessionID)
		if err != nil {
			return nil, err
		}
		appState.ReplaceMessages(session.BuildConversation(loaded.Events))
	}

	if err := sessions.appendUserMessage(prompt); err != nil {
		return nil, err
	}
	appState.AddMessage(llm.RoleUser, prompt)

	eventCh, err := appState.StreamChat(ctx, opts.Config, llm.StreamOptions{SessionID: sessions.currentID()})
	if err != nil {
		return nil, err
	}
	if eventCh == nil {
		return nil, fmt.Errorf("LLM client not initialized")
	}

	handler := NewStreamHandler(nil)
	handler.workingDir = opts.WorkingDir
	handler.showThinking = false
	handler.Start(eventCh, "")
	turnMemory := newTurnMemoryAccumulator()

	var lastUsage *llm.TokenUsage
	for {
		select {
		case diffReq := <-diffEmitter.GetDiffChan():
			handler.HandleDiff(diffReq.Lines)
			close(diffReq.Done)
		case event, ok := <-eventCh:
			if !ok {
				return finishHeadlessRun(opts.Out, format, sessions, handler, turnMemory, lastUsage)
			}
			switch event.Type {
			case llm.StreamEventTypeChunk:
				handler.HandleChunk(event.Content)
			case llm.StreamEventTypeReasoningChunk:
				handler.HandleReasoningChunk(event.Content)
			case llm.StreamEventTypeToolStart:
				handleHeadlessToolStart(handler, event.ToolCall)
			case llm.StreamEventTypeToolEnd:
				turnMemory.RecordToolEnd(cloneToolCallWithRelativePath(event.ToolCall, opts.WorkingDir))
				handleHeadlessToolEnd(handler, event.ToolCall)
			case llm.StreamEventTypeUsage:
				lastUsage = event.Usage
			case llm.StreamEventTypeRetry:
				handler.RewindForRetry()
			case llm.StreamEventTypeDone:
				return finishHeadlessRun(opts.Out, format, sessions, handler, turnMemory, lastUsage)
			case llm.StreamEventTypeIncomplete:
				return failHeadlessRun(sessions, handler, turnMemory, event.Error)
			case llm.StreamEventTypeError:
				return failHeadlessRun(sessions, handler, turnMemory, event.Error)
			}
		case <-ctx.Done():
			return failHeadlessRun(sessions, handler, turnMemory, ctx.Err())
		}
	}
}

func loadHeadlessSession(sessions *replSessionState, sessionID string) (*session.LoadedSession, error) {
	summaries, err := sessions.listSessions()
	if err != nil {
		return nil, err
	}
	for _, summary := range summaries {
		if summary.ID == sessionID {
			return sessions.load(summary)
		}
	}
	return nil, fmt.Errorf("session %q not found", sessionID)
}

func handleHeadlessToolStart(handler *StreamHandler, toolCall *llm.ToolCall) {
	if toolCall == nil {
		return
	}
	if toolCall.Name == "bash" {
		command, _ := toolCall.Input["command"].(string)
		summary, _ := toolCall.Input["summary"].(string)
		handler.HandleBashStart(command, summary)
		return
	}
	handler.HandleToolStart(toolCall)
}

func handleHeadlessToolEnd(handler *StreamHandler, toolCall *llm.ToolCall) {
	if toolCall == nil {
		return
	}
	if toolCall.Name == "bash" {
		handler.HandleBashEnd(toolCall)
		return
	}
	handler.HandleToolEnd(toolCall)
}

func finishHeadlessRun(out io.Writer, format string, sessions *replSessionState, handler *StreamHandler, turnMemory *turnMemoryAccumulator, usage *llm.TokenUsage) (*HeadlessRunResult, error) {
	segments := cloneStreamSegments(handler.segments)
	_, response := handler.HandleDone()
	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    response,
		TurnMemory: turnMemory.Build(),
	}
	if err := sessions.appendAssistantTurn(segments, assistantMessage, false, ""); err != nil {
		return nil, err
	}

	result := &HeadlessRunResult{
		SessionID:         sessions.currentID(),
		OpenCodeSessionID: strings.ReplaceAll(sessions.currentID(), "-", ""),
		Text:              response,
		Usage:             cloneHeadlessUsage(usage),
	}
	return result, writeHeadlessResult(out, format, result)
}

func failHeadlessRun(sessions *replSessionState, handler *StreamHandler, turnMemory *turnMemoryAccumulator, err error) (*HeadlessRunResult, error) {
	if err == nil {
		err = fmt.Errorf("LLM stream incomplete")
	}
	segments := cloneStreamSegments(handler.segments)
	partialResponse := handler.GetResponse()
	_, errMsg := handler.HandleError(err)
	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    partialResponse,
		TurnMemory: turnMemory.Build(),
	}
	_ = sessions.appendAssistantTurn(segments, assistantMessage, false, errMsg)
	return nil, err
}

func cloneHeadlessUsage(usage *llm.TokenUsage) *headlessUsage {
	if usage == nil {
		return nil
	}
	return &headlessUsage{
		InputTokens:     usage.InputTokens,
		OutputTokens:    usage.OutputTokens,
		ReasoningTokens: usage.ReasoningTokens,
		TotalTokens:     usage.TotalTokens,
		CachedTokens:    usage.CachedTokens,
	}
}

func writeHeadlessResult(out io.Writer, format string, result *HeadlessRunResult) error {
	if out == nil {
		return nil
	}
	switch format {
	case HeadlessFormatJSON:
		encoder := json.NewEncoder(out)
		return encoder.Encode(result)
	default:
		if result.Text == "" {
			_, err := fmt.Fprintln(out)
			return err
		}
		_, err := fmt.Fprintln(out, result.Text)
		return err
	}
}
