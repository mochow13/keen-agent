package subagents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/tools"
)

type ClientFactory func(*config.ResolvedConfig) (llm.LLMClient, error)

const defaultTimeoutSeconds = 600

type ProfileProvider func() []Profile

type Runner struct {
	WorkingDir  string
	Config      *config.ResolvedConfig
	Profiles    []Profile
	GetProfiles ProfileProvider
	NewClient   ClientFactory
	Registry    *tools.Registry
}

type Result struct {
	Agent  string `json:"agent"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (r *Runner) RunDelegate(ctx context.Context, agent, task string, timeoutSeconds int) (any, error) {
	return r.Run(ctx, agent, task, timeoutSeconds)
}

func (r *Runner) Run(ctx context.Context, agent, task string, timeoutSeconds int) (Result, error) {
	profile, ok := Find(r.profiles(), strings.TrimSpace(agent))
	if !ok {
		return Result{Agent: agent, Status: "error", Error: "unknown subagent"}, fmt.Errorf("unknown subagent %q", agent)
	}
	if strings.TrimSpace(task) == "" {
		return Result{Agent: profile.Name, Status: "error", Error: "task is required"}, fmt.Errorf("task is required")
	}
	if r.Config == nil {
		return Result{Agent: profile.Name, Status: "error", Error: "LLM config not initialized"}, fmt.Errorf("LLM config not initialized")
	}
	if r.NewClient == nil {
		r.NewClient = llm.NewClient
	}

	client, err := r.NewClient(cloneConfig(r.Config))
	if err != nil {
		return Result{Agent: profile.Name, Status: "error", Error: err.Error()}, err
	}

	childCtx := ctx
	cancel := func() {}
	timeoutSeconds = effectiveTimeoutSeconds(timeoutSeconds, profile.TimeoutSeconds)
	if timeoutSeconds > 0 {
		childCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	}
	defer cancel()

	messages := []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: buildChildPrompt(r.WorkingDir, profile),
		},
		{
			Role:    llm.RoleUser,
			Content: buildUserTask(task),
		},
	}
	events, err := client.StreamChat(childCtx, messages, r.toolRegistry(profile), llm.StreamOptions{OneShot: true})
	if err != nil {
		return Result{Agent: profile.Name, Status: "error", Error: err.Error()}, err
	}
	text, err := collectResult(childCtx, events)
	if err != nil {
		return Result{Agent: profile.Name, Status: "error", Result: text, Error: err.Error()}, err
	}
	return Result{Agent: profile.Name, Status: "completed", Result: strings.TrimSpace(text)}, nil
}

func (r *Runner) toolRegistry(profile Profile) *tools.Registry {
	child := tools.NewRegistry()
	if r.Registry == nil {
		return child
	}
	for _, name := range readOnlyTools(profile) {
		if tool, ok := r.Registry.Get(name); ok {
			_ = child.Register(tool)
		}
	}
	return child
}

func (r *Runner) profiles() []Profile {
	if r.GetProfiles != nil {
		return r.GetProfiles()
	}
	return append([]Profile(nil), r.Profiles...)
}

func effectiveTimeoutSeconds(requested, profile int) int {
	if requested > 0 {
		return requested
	}
	if profile > 0 {
		return profile
	}
	return defaultTimeoutSeconds
}

func collectResult(ctx context.Context, events <-chan llm.StreamEvent) (string, error) {
	var sb strings.Builder
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return strings.TrimSpace(sb.String()), nil
			}
			switch event.Type {
			case llm.StreamEventTypeChunk:
				sb.WriteString(event.Content)
			case llm.StreamEventTypeDone:
				return strings.TrimSpace(sb.String()), nil
			case llm.StreamEventTypeError, llm.StreamEventTypeIncomplete:
				if event.Error != nil {
					return strings.TrimSpace(sb.String()), event.Error
				}
				return strings.TrimSpace(sb.String()), fmt.Errorf("subagent stream incomplete")
			}
		case <-ctx.Done():
			return strings.TrimSpace(sb.String()), ctx.Err()
		}
	}
}

func buildChildPrompt(workingDir string, profile Profile) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(profile.Instructions))
	sb.WriteString(fmt.Sprintf("\n\nWorking directory: %s", workingDir))
	return sb.String()
}

func buildUserTask(task string) string {
	var sb strings.Builder
	sb.WriteString("Delegated task:\n")
	sb.WriteString(strings.TrimSpace(task))
	return sb.String()
}

func cloneConfig(cfg *config.ResolvedConfig) *config.ResolvedConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	return &cloned
}
