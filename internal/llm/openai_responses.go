package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

type responseStream interface {
	Next() bool
	Current() responses.ResponseStreamEventUnion
	Err() error
	Close() error
}

type responseStreamFactory func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) responseStream

type sdkResponseStream struct {
	stream *ssestream.Stream[responses.ResponseStreamEventUnion]
}

func (s *sdkResponseStream) Next() bool {
	return s.stream.Next()
}

func (s *sdkResponseStream) Current() responses.ResponseStreamEventUnion {
	return s.stream.Current()
}

func (s *sdkResponseStream) Err() error {
	return s.stream.Err()
}

func (s *sdkResponseStream) Close() error {
	return s.stream.Close()
}

type OpenAIResponsesClient struct {
	provider                Provider
	model                   string
	thinkingEffort          string
	maxRetries              int
	client                  openai.Client
	responseStreamImpl      responseStreamFactory
	pendingState            []responses.ResponseInputItemUnionParam
	contextWindowTokenCount int
}

func NewOpenAIResponsesClient(cfg *ClientConfig) (*OpenAIResponsesClient, error) {
	if cfg.Provider != Provider(config.ProviderOpenAI) {
		return nil, fmt.Errorf("unsupported Responses API provider: %s. %s", cfg.Provider, config.ConfigFixHint)
	}

	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := openai.NewClient(opts...)

	c := &OpenAIResponsesClient{
		provider:                cfg.Provider,
		model:                   cfg.Model,
		thinkingEffort:          cfg.ThinkingEffort,
		maxRetries:              retryCount(cfg.MaxRetries),
		client:                  client,
		contextWindowTokenCount: cfg.ContextWindowTokens,
	}
	c.responseStreamImpl = func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) responseStream {
		return &sdkResponseStream{stream: c.client.Responses.NewStreaming(ctx, params, opts...)}
	}

	return c, nil
}

func toOpenAIResponseInput(messages []Message) []responses.ResponseInputItemUnionParam {
	result := make([]responses.ResponseInputItemUnionParam, 0, len(messages))
	for _, m := range messages {
		content := FormatMessageForProvider(m)
		switch m.Role {
		case RoleSystem:
			result = append(result, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleSystem))
		case RoleUser:
			result = append(result, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleUser))
		case RoleAssistant:
			result = append(result, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleAssistant))
		}
	}
	return result
}

func toOpenAIResponseTools(registry *tools.Registry) []responses.ToolUnionParam {
	if registry == nil {
		return nil
	}

	all := registry.All()
	result := make([]responses.ToolUnionParam, 0, len(all))
	for _, t := range all {
		result = append(result, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        t.Name(),
				Description: param.NewOpt(t.Description()),
				Parameters:  t.InputSchema(),
				Strict:      param.NewOpt(false),
			},
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func reasoningEffortForLevel(effort string) shared.ReasoningEffort {
	switch effort {
	case "low":
		return shared.ReasoningEffortLow
	case "medium":
		return shared.ReasoningEffortMedium
	case "high":
		return shared.ReasoningEffortHigh
	case "xhigh":
		return shared.ReasoningEffort("xhigh")
	case "none":
		return shared.ReasoningEffort("none")
	default:
		return ""
	}
}

func (c *OpenAIResponsesClient) StreamChat(
	ctx context.Context,
	messages []Message,
	toolRegistry *tools.Registry,
	opts ...StreamOptions,
) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent)

	go func() {
		defer close(eventCh)

		input := toOpenAIResponseInput(messages)
		oneShot := streamOptions(opts).OneShot
		var replayedPendingInput []responses.ResponseInputItemUnionParam
		if !oneShot {
			input, replayedPendingInput = c.injectPendingState(input)
		}
		turnStartLen := len(input)
		responseTools := toOpenAIResponseTools(toolRegistry)

		for range maxToolTurns {
			reducedInput, reduction := reduceResponsesContextForRequest(c.contextWindowTokenCount, input)
			if !reduction.FitsBudget {
				slog.Debug("OpenAI Responses context still exceeds budget after reduction", "inputTokenCount", reduction.ReducedTokenCount, "removedToolResultCount", reduction.RemovedToolResults)
				c.pendingState = nil
				c.emitTerminalEvent(eventCh, input, turnStartLen, replayedPendingInput, fmt.Errorf(contextWindowExceededError))
				return
			}
			input = reducedInput

			params := responses.ResponseNewParams{
				Model: c.model,
				Store: param.NewOpt(false),
				Input: responses.ResponseNewParamsInputUnion{
					OfInputItemList: input,
				},
			}
			if c.thinkingEffort != "" && c.thinkingEffort != "off" {
				params.Reasoning = shared.ReasoningParam{
					Effort: reasoningEffortForLevel(c.thinkingEffort),
				}
			}
			if len(responseTools) > 0 {
				params.Tools = responseTools
			}

			completed, streamedContent, toolCalls, err := c.collectTurnWithRetry(ctx, params, eventCh)
			if err != nil {
				c.exitIncomplete(eventCh, input, turnStartLen, replayedPendingInput, err, oneShot)
				return
			}
			if completed == nil {
				c.exitIncomplete(eventCh, input, turnStartLen, replayedPendingInput, nil, oneShot)
				return
			}

			if completed.Usage.InputTokens > 0 || completed.Usage.OutputTokens > 0 {
				slog.Debug(
					"OpenAI Responses usage",
					"inputTokens", completed.Usage.InputTokens,
					"outputTokens", completed.Usage.OutputTokens,
					"totalTokens", completed.Usage.TotalTokens,
					"reasoningTokens", completed.Usage.OutputTokensDetails.ReasoningTokens,
					"cachedTokens", completed.Usage.InputTokensDetails.CachedTokens,
				)
				eventCh <- StreamEvent{
					Type: StreamEventTypeUsage,
					Usage: &TokenUsage{
						InputTokens:     int(completed.Usage.InputTokens),
						OutputTokens:    int(completed.Usage.OutputTokens),
						TotalTokens:     int(completed.Usage.TotalTokens),
						ReasoningTokens: int(completed.Usage.OutputTokensDetails.ReasoningTokens),
						CachedTokens:    int(completed.Usage.InputTokensDetails.CachedTokens),
					},
				}
			}
			emitMissingFinalContent(eventCh, completed.OutputText(), streamedContent)

			if len(toolCalls) == 0 {
				eventCh <- StreamEvent{Type: StreamEventTypeDone}
				return
			}

			input = append(input, responseOutputInputs(completed.Output, toolCalls, streamedContent)...)
			input = append(input, c.executeTools(ctx, toolCalls, toolRegistry, eventCh)...)
		}

		c.exitIncomplete(eventCh, input, turnStartLen, replayedPendingInput, nil, oneShot)
	}()

	return eventCh, nil
}

func (c *OpenAIResponsesClient) Reset() {
	c.pendingState = nil
}

func (c *OpenAIResponsesClient) injectPendingState(input []responses.ResponseInputItemUnionParam) ([]responses.ResponseInputItemUnionParam, []responses.ResponseInputItemUnionParam) {
	if len(c.pendingState) == 0 {
		return input, nil
	}

	replayedPendingInput := append([]responses.ResponseInputItemUnionParam(nil), c.pendingState...)

	slog.Debug("Injecting pending state", "pending_messages", len(c.pendingState), "total_messages", len(input))
	if prettyJSON, err := json.MarshalIndent(c.pendingState, "", "  "); err == nil {
		slog.Debug("Pending state contents:\n" + string(prettyJSON))
	}

	if len(input) > 0 {
		last := input[len(input)-1]
		input = append(input[:len(input)-1], replayedPendingInput...)
		input = append(input, last)
	} else {
		input = append(input, replayedPendingInput...)
	}
	c.pendingState = nil
	return input, replayedPendingInput
}

func (c *OpenAIResponsesClient) exitIncomplete(eventCh chan<- StreamEvent, input []responses.ResponseInputItemUnionParam, turnStartLen int, replayedPendingInput []responses.ResponseInputItemUnionParam, err error, oneShot bool) {
	if !oneShot {
		c.savePendingIfAccumulated(input, turnStartLen, replayedPendingInput)
	}
	c.emitTerminalEvent(eventCh, input, turnStartLen, replayedPendingInput, err)
}

func (c *OpenAIResponsesClient) savePendingIfAccumulated(input []responses.ResponseInputItemUnionParam, turnStartLen int, replayedPendingInput []responses.ResponseInputItemUnionParam) {
	if len(replayedPendingInput) == 0 && len(input) <= turnStartLen {
		return
	}

	newDelta := []responses.ResponseInputItemUnionParam(nil)
	if len(input) > turnStartLen {
		newDelta = input[turnStartLen:]
	}

	c.pendingState = make([]responses.ResponseInputItemUnionParam, 0, len(replayedPendingInput)+len(newDelta))
	c.pendingState = append(c.pendingState, replayedPendingInput...)
	c.pendingState = append(c.pendingState, newDelta...)
}

func (c *OpenAIResponsesClient) emitTerminalEvent(eventCh chan<- StreamEvent, input []responses.ResponseInputItemUnionParam, turnStartLen int, replayedPendingInput []responses.ResponseInputItemUnionParam, err error) {
	if len(replayedPendingInput) > 0 || len(input) > turnStartLen {
		eventCh <- StreamEvent{Type: StreamEventTypeIncomplete, Error: err}
	} else if err != nil {
		eventCh <- StreamEvent{Type: StreamEventTypeError, Error: err}
	} else {
		eventCh <- StreamEvent{Type: StreamEventTypeDone}
	}
}

func (c *OpenAIResponsesClient) collectTurnWithRetry(
	ctx context.Context,
	params responses.ResponseNewParams,
	eventCh chan<- StreamEvent,
) (*responses.Response, string, []responses.ResponseFunctionToolCall, error) {
	maxRetries := retryCount(c.maxRetries)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		completed, streamedContent, toolCalls, err := c.collectTurn(ctx, params, eventCh)
		if err == nil {
			return completed, streamedContent, toolCalls, nil
		}
		if !isRetryableError(err) || attempt == maxRetries {
			return nil, "", nil, err
		}

		backoff := time.Duration(attempt) * time.Second
		slog.Debug("LLM stream error, retrying", "attempt", attempt, "maxRetries", maxRetries, "backoff", backoff, "error", err)
		eventCh <- StreamEvent{Type: StreamEventTypeRetry, Error: err, Attempt: attempt}

		select {
		case <-ctx.Done():
			return nil, "", nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, "", nil, nil
}

func (c *OpenAIResponsesClient) collectTurn(
	ctx context.Context,
	params responses.ResponseNewParams,
	eventCh chan<- StreamEvent,
) (*responses.Response, string, []responses.ResponseFunctionToolCall, error) {
	stream := c.responseStreamImpl(ctx, params)
	var completed *responses.Response
	var streamedContent strings.Builder

	for stream.Next() {
		ev := stream.Current()

		switch ev.Type {
		case "response.output_text.delta":
			if ev.Delta.OfString != "" {
				streamedContent.WriteString(ev.Delta.OfString)
				emitChunk(eventCh, ev.Delta.OfString)
			}
		case "response.reasoning.delta", "response.reasoning_summary.delta", "response.reasoning_summary_text.delta":
			reasoning := ev.Delta.OfString
			if reasoning == "" {
				reasoning = ev.Text
			}
			if reasoning != "" {
				eventCh <- StreamEvent{
					Type:    StreamEventTypeReasoningChunk,
					Content: reasoning,
				}
			}
		case "error":
			msg := strings.TrimSpace(ev.Message)
			if msg == "" {
				msg = "responses stream error"
			}
			if ev.Code != "" {
				msg = msg + " (" + ev.Code + ")"
			}
			return nil, streamedContent.String(), nil, fmt.Errorf("%s", msg)
		case "response.completed":
			v := ev.AsResponseCompleted()
			completed = &v.Response
		}
	}
	_ = stream.Close()

	if err := stream.Err(); err != nil {
		return nil, streamedContent.String(), nil, fmt.Errorf("stream error: %w", err)
	}
	if completed == nil {
		return nil, streamedContent.String(), nil, nil
	}

	toolCalls := make([]responses.ResponseFunctionToolCall, 0)
	for _, item := range completed.Output {
		if item.Type != "function_call" {
			continue
		}
		toolCalls = append(toolCalls, item.AsFunctionCall())
	}

	return completed, streamedContent.String(), toolCalls, nil
}

func responseOutputInputs(output []responses.ResponseOutputItemUnion, fallbackToolCalls []responses.ResponseFunctionToolCall, fallbackText string) []responses.ResponseInputItemUnionParam {
	result := make([]responses.ResponseInputItemUnionParam, 0, len(output)+len(fallbackToolCalls)+1)
	seenMessage := false
	seenToolCalls := make(map[string]bool)
	for _, item := range output {
		switch item.Type {
		case "message":
			message := item.AsMessage()
			if len(message.Content) == 0 {
				continue
			}
			messageParam := message.ToParam()
			result = append(result, responses.ResponseInputItemUnionParam{OfOutputMessage: &messageParam})
			seenMessage = true
		case "function_call":
			toolCall := item.AsFunctionCall()
			result = append(result, responseFunctionCallInput(toolCall))
			markResponseToolCallSeen(seenToolCalls, toolCall)
		}
	}
	if !seenMessage && fallbackText != "" {
		result = append(result, responses.ResponseInputItemParamOfMessage(fallbackText, responses.EasyInputMessageRoleAssistant))
	}
	for _, tc := range fallbackToolCalls {
		if responseToolCallSeen(seenToolCalls, tc) {
			continue
		}
		result = append(result, responseFunctionCallInput(tc))
	}
	return result
}

func responseFunctionCallInput(tc responses.ResponseFunctionToolCall) responses.ResponseInputItemUnionParam {
	item := responses.ResponseInputItemParamOfFunctionCall(tc.Arguments, tc.CallID, tc.Name)
	if tc.ID != "" {
		item.OfFunctionCall.ID = param.NewOpt(tc.ID)
	}
	if tc.Status != "" {
		item.OfFunctionCall.Status = tc.Status
	}
	return item
}

func markResponseToolCallSeen(seen map[string]bool, tc responses.ResponseFunctionToolCall) {
	key := responseToolCallKey(tc)
	if key != "" {
		seen[key] = true
	}
}

func responseToolCallSeen(seen map[string]bool, tc responses.ResponseFunctionToolCall) bool {
	key := responseToolCallKey(tc)
	return key != "" && seen[key]
}

func responseToolCallKey(tc responses.ResponseFunctionToolCall) string {
	if tc.CallID != "" {
		return tc.CallID
	}
	return tc.ID
}

func (c *OpenAIResponsesClient) executeTools(
	ctx context.Context,
	toolCalls []responses.ResponseFunctionToolCall,
	registry *tools.Registry,
	eventCh chan<- StreamEvent,
) []responses.ResponseInputItemUnionParam {
	toolMessages := make([]responses.ResponseInputItemUnionParam, 0, len(toolCalls))

	for _, tc := range toolCalls {
		start := time.Now()
		input := map[string]any{}
		if tc.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
				input = map[string]any{}
			}
		}
		slog.Debug("Tool request", "tool", tc.Name, "input", input)
		eventCh <- StreamEvent{
			Type: StreamEventTypeToolStart,
			ToolCall: &ToolCall{
				Name:  tc.Name,
				Input: input,
			},
		}

		var output any
		var execErr error

		if registry == nil {
			execErr = fmt.Errorf("tool registry not available")
		} else if tool, exists := registry.Get(tc.Name); !exists {
			execErr = fmt.Errorf("tool %q not found", tc.Name)
		} else {
			output, execErr = tool.Execute(ctx, input)
		}

		duration := time.Since(start)
		toolCall := &ToolCall{
			Name:     tc.Name,
			Input:    input,
			Output:   output,
			Duration: duration,
		}

		var toolOutput string
		if execErr != nil {
			toolCall.Error = execErr.Error()
			slog.Debug("Tool response", "tool", tc.Name, "error", execErr.Error(), "duration", duration)
			eventCh <- StreamEvent{
				Type:     StreamEventTypeToolEnd,
				ToolCall: toolCall,
			}
			toolOutput = fmt.Sprintf(`{"error":%q}`, execErr.Error())
		} else {
			slog.Debug("Tool response", "tool", tc.Name, "duration", duration)
			eventCh <- StreamEvent{
				Type:     StreamEventTypeToolEnd,
				ToolCall: toolCall,
			}
			if output == nil {
				output = map[string]any{}
			}
			b, err := json.Marshal(output)
			if err != nil {
				toolOutput = "{}"
			} else {
				toolOutput = string(b)
			}
		}

		toolMessages = append(toolMessages, responses.ResponseInputItemParamOfFunctionCallOutput(tc.CallID, toolOutput))
	}

	return toolMessages
}
