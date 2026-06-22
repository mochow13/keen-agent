package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/tools"
	"google.golang.org/genai"
)

const maxToolTurns = 5000

type streamFunc func(ctx context.Context, g *genkit.Genkit, opts ...ai.GenerateOption) iter.Seq2[*ai.ModelStreamValue, error]

type GenkitClient struct {
	g                       *genkit.Genkit
	provider                Provider
	model                   string
	thinkingEffort          string
	maxRetries              int
	streamImpl              streamFunc
	pendingState            []*ai.Message
	contextWindowTokenCount int
	headers                 map[string]string
}

func NewGenkitClient(cfg *ClientConfig) (*GenkitClient, error) {
	ctx := context.Background()

	var g *genkit.Genkit
	var modelName string

	switch cfg.Provider {
	case config.ProviderGoogleAI:
		g = genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{
			APIKey: cfg.APIKey,
		}))
		modelName = "googleai/" + cfg.Model
	default:
		return nil, fmt.Errorf("unsupported provider in config: %s. %s", cfg.Provider, config.ConfigFixHint)
	}

	if g == nil {
		return nil, fmt.Errorf("failed to initialize genkit")
	}

	return &GenkitClient{
		g:                       g,
		provider:                cfg.Provider,
		model:                   modelName,
		thinkingEffort:          cfg.ThinkingEffort,
		maxRetries:              retryCount(cfg.MaxRetries),
		contextWindowTokenCount: cfg.ContextWindowTokens,
		streamImpl:              genkit.GenerateStream,
		headers:                 cfg.Headers,
	}, nil
}

func toGenkitRole(role Role) ai.Role {
	switch role {
	case RoleUser:
		return ai.RoleUser
	case RoleAssistant:
		return ai.RoleModel
	case RoleSystem:
		return ai.RoleSystem
	default:
		return ai.Role(role)
	}
}

func toGenkitMessages(messages []Message) []*ai.Message {
	aiMessages := make([]*ai.Message, len(messages))
	for i, m := range messages {
		content := FormatMessageForProvider(m)
		aiMessages[i] = &ai.Message{
			Role: toGenkitRole(m.Role),
			Content: []*ai.Part{
				ai.NewTextPart(content),
			},
		}
	}
	return aiMessages
}

func budgetForEffort(effort string) *int32 {
	var b int32
	switch effort {
	case "low":
		b = 1024
	case "medium":
		b = 8192
	case "high":
		b = 24576
	default:
		return nil
	}
	return &b
}

func buildGenkitGenerateConfig(thinkingEffort string, provider Provider, headers map[string]string) *genai.GenerateContentConfig {
	var cfg *genai.GenerateContentConfig
	if thinkingEffort != "" && thinkingEffort != "off" && provider == Provider(config.ProviderGoogleAI) {
		budget := budgetForEffort(thinkingEffort)
		if budget != nil {
			cfg = &genai.GenerateContentConfig{
				ThinkingConfig: &genai.ThinkingConfig{
					IncludeThoughts: true,
					ThinkingBudget:  budget,
				},
			}
		}
	}
	if len(headers) > 0 {
		if cfg == nil {
			cfg = &genai.GenerateContentConfig{}
		}
		cfg.HTTPOptions = &genai.HTTPOptions{
			Headers: make(http.Header, len(headers)),
		}
		for k, v := range headers {
			cfg.HTTPOptions.Headers.Set(k, v)
		}
	}
	return cfg
}

func (c *GenkitClient) collectTurnWithRetry(
	ctx context.Context,
	opts []ai.GenerateOption,
	eventCh chan<- StreamEvent,
) (*ai.ModelResponse, error) {
	maxRetries := retryCount(c.maxRetries)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		modelResponse, err := c.collectTurn(ctx, opts, eventCh)
		if err == nil {
			return modelResponse, nil
		}
		if !isRetryableError(err) || attempt == maxRetries {
			return nil, err
		}

		backoff := time.Duration(attempt) * time.Second
		slog.Debug("LLM stream error, retrying", "attempt", attempt, "maxRetries", maxRetries, "backoff", backoff, "error", err)
		eventCh <- StreamEvent{Type: StreamEventTypeRetry, Error: err, Attempt: attempt}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, nil
}

func (c *GenkitClient) collectTurn(
	ctx context.Context,
	opts []ai.GenerateOption,
	eventCh chan<- StreamEvent,
) (*ai.ModelResponse, error) {
	stream := c.streamImpl(ctx, c.g, opts...)
	var modelResponse *ai.ModelResponse

	for result, err := range stream {
		if err != nil {
			return nil, err
		}

		if result.Done {
			modelResponse = result.Response
			break
		}

		if result.Chunk != nil && len(result.Chunk.Content) > 0 {
			for _, part := range result.Chunk.Content {
				if part.IsReasoning() && part.Text != "" {
					eventCh <- StreamEvent{
						Type:    StreamEventTypeReasoningChunk,
						Content: part.Text,
					}
				} else if (part.IsText() || part.IsData()) && part.Text != "" {
					eventCh <- StreamEvent{
						Type:    StreamEventTypeChunk,
						Content: part.Text,
					}
				}
			}
		}
	}

	return modelResponse, nil
}

func (c *GenkitClient) StreamChat(
	ctx context.Context,
	messages []Message,
	toolRegistry *tools.Registry,
	opts ...StreamOptions,
) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent)

	go func() {
		defer close(eventCh)

		aiMessages := toGenkitMessages(messages)
		oneShot := streamOptions(opts).OneShot
		var injectedPending []*ai.Message
		if !oneShot {
			aiMessages, injectedPending = c.injectPendingState(aiMessages)
		}
		turnStartLen := len(aiMessages)

		var genkitTools []ai.ToolRef
		if toolRegistry != nil && toolRegistry.Count() > 0 {
			genkitTools = ToGenkitTools(toolRegistry)
		}

		for range maxToolTurns {
			reducedMessages, reduction := reduceGenkitContextForRequest(c.contextWindowTokenCount, aiMessages)
			if !reduction.FitsBudget {
				slog.Debug("Genkit context still exceeds budget after reduction", "inputTokenCount", reduction.ReducedTokenCount, "removedToolResultCount", reduction.RemovedToolResults)
				c.pendingState = nil
				c.emitTerminalEvent(eventCh, aiMessages, turnStartLen, injectedPending, fmt.Errorf(contextWindowExceededError))
				return
			}
			aiMessages = reducedMessages

			opts := []ai.GenerateOption{
				ai.WithModelName(c.model),
				ai.WithMessages(aiMessages...),
			}

			if genCfg := buildGenkitGenerateConfig(c.thinkingEffort, c.provider, c.headers); genCfg != nil {
				opts = append(opts, ai.WithConfig(genCfg))
			}

			if len(genkitTools) > 0 {
				opts = append(opts, ai.WithTools(genkitTools...))
				opts = append(opts, ai.WithReturnToolRequests(true))
			}

			modelResponse, err := c.collectTurnWithRetry(ctx, opts, eventCh)
			if err != nil {
				c.exitIncomplete(eventCh, aiMessages, turnStartLen, injectedPending, err, oneShot)
				return
			}

			if modelResponse == nil || modelResponse.Message == nil {
				c.exitIncomplete(eventCh, aiMessages, turnStartLen, injectedPending, nil, oneShot)
				return
			}

			if modelResponse.Usage != nil && (modelResponse.Usage.InputTokens > 0 || modelResponse.Usage.OutputTokens > 0) {
				eventCh <- StreamEvent{
					Type: StreamEventTypeUsage,
					Usage: &TokenUsage{
						InputTokens:  modelResponse.Usage.InputTokens,
						OutputTokens: modelResponse.Usage.OutputTokens,
						TotalTokens:  modelResponse.Usage.TotalTokens,
					},
				}
			}

			toolRequests := modelResponse.ToolRequests()
			if len(toolRequests) == 0 {
				eventCh <- StreamEvent{Type: StreamEventTypeDone}
				return
			}

			aiMessages = append(aiMessages, modelResponse.Message)

			toolResponseParts := c.executeTools(ctx, toolRequests, toolRegistry, eventCh)
			if len(toolResponseParts) > 0 {
				toolMsg := &ai.Message{
					Role:    ai.RoleTool,
					Content: toolResponseParts,
				}
				aiMessages = append(aiMessages, toolMsg)
			}
		}

		c.exitIncomplete(eventCh, aiMessages, turnStartLen, injectedPending, nil, oneShot)
	}()

	return eventCh, nil
}

func (c *GenkitClient) Reset() {
	c.pendingState = nil
}

func (c *GenkitClient) injectPendingState(aiMessages []*ai.Message) ([]*ai.Message, []*ai.Message) {
	if len(c.pendingState) == 0 {
		return aiMessages, nil
	}

	injectedPending := append([]*ai.Message(nil), c.pendingState...)

	slog.Debug("Injecting pending state", "pending_messages", len(c.pendingState), "total_messages", len(aiMessages))

	if len(aiMessages) > 0 {
		last := aiMessages[len(aiMessages)-1]
		aiMessages = append(aiMessages[:len(aiMessages)-1], injectedPending...)
		aiMessages = append(aiMessages, last)
	} else {
		aiMessages = append(aiMessages, injectedPending...)
	}
	c.pendingState = nil
	return aiMessages, injectedPending
}

func (c *GenkitClient) savePendingIfAccumulated(aiMessages []*ai.Message, turnStartLen int, injectedPending []*ai.Message) {
	if len(injectedPending) == 0 && len(aiMessages) <= turnStartLen {
		return
	}

	newDelta := []*ai.Message(nil)
	if len(aiMessages) > turnStartLen {
		newDelta = aiMessages[turnStartLen:]
	}

	c.pendingState = make([]*ai.Message, 0, len(injectedPending)+len(newDelta))
	c.pendingState = append(c.pendingState, injectedPending...)
	c.pendingState = append(c.pendingState, newDelta...)
}

func (c *GenkitClient) emitTerminalEvent(eventCh chan<- StreamEvent, aiMessages []*ai.Message, turnStartLen int, injectedPending []*ai.Message, err error) {
	if len(injectedPending) > 0 || len(aiMessages) > turnStartLen {
		eventCh <- StreamEvent{Type: StreamEventTypeIncomplete, Error: err}
	} else if err != nil {
		eventCh <- StreamEvent{Type: StreamEventTypeError, Error: err}
	} else {
		eventCh <- StreamEvent{Type: StreamEventTypeDone}
	}
}

func (c *GenkitClient) exitIncomplete(eventCh chan<- StreamEvent, aiMessages []*ai.Message, turnStartLen int, injectedPending []*ai.Message, err error, oneShot bool) {
	if !oneShot {
		c.savePendingIfAccumulated(aiMessages, turnStartLen, injectedPending)
	}
	c.emitTerminalEvent(eventCh, aiMessages, turnStartLen, injectedPending, err)
}

func (c *GenkitClient) executeTools(
	ctx context.Context,
	toolRequests []*ai.ToolRequest,
	registry *tools.Registry,
	eventCh chan<- StreamEvent,
) []*ai.Part {
	var toolResponseParts []*ai.Part

	for _, req := range toolRequests {
		start := time.Now()

		input, _ := req.Input.(map[string]any)
		if input == nil {
			if raw, ok := req.Input.(json.RawMessage); ok {
				if err := json.Unmarshal(raw, &input); err != nil {
					input = nil
				}
			}
		}
		slog.Debug("Tool request", "tool", req.Name, "input", input)
		eventCh <- StreamEvent{
			Type: StreamEventTypeToolStart,
			ToolCall: &ToolCall{
				Name:  req.Name,
				Input: input,
			},
		}

		var output any
		var execErr error

		if registry == nil {
			execErr = fmt.Errorf("tool registry not available")
		} else if tool, exists := registry.Get(req.Name); !exists {
			execErr = fmt.Errorf("tool %q not found", req.Name)
		} else {
			output, execErr = tool.Execute(ctx, input)
		}

		duration := time.Since(start)

		toolCall := &ToolCall{
			Name:     req.Name,
			Input:    input,
			Output:   output,
			Duration: duration,
		}

		if execErr != nil {
			toolCall.Error = execErr.Error()
			slog.Debug("Tool response", "tool", req.Name, "error", execErr.Error(), "duration", duration)
			eventCh <- StreamEvent{
				Type:     StreamEventTypeToolEnd,
				ToolCall: toolCall,
			}
			toolResponseParts = append(toolResponseParts, ai.NewToolResponsePart(&ai.ToolResponse{
				Name:   req.Name,
				Ref:    req.Ref,
				Output: map[string]any{"error": execErr.Error()},
			}))
		} else {
			slog.Debug("Tool response", "tool", req.Name, "duration", duration)
			eventCh <- StreamEvent{
				Type:     StreamEventTypeToolEnd,
				ToolCall: toolCall,
			}
			if output == nil {
				output = map[string]any{}
			}
			toolResponseParts = append(toolResponseParts, ai.NewToolResponsePart(&ai.ToolResponse{
				Name:   req.Name,
				Ref:    req.Ref,
				Output: output,
			}))
		}
	}

	return toolResponseParts
}
