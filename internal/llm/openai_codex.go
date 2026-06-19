package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/mochow13/keen-agent/internal/auth"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

const openAICodexBaseURL = "https://chatgpt.com/backend-api/codex/"

type OpenAICodexClient struct {
	model                   string
	thinkingEffort          string
	maxRetries              int
	client                  openai.Client
	responseStreamImpl      responseStreamFactory
	authManager             *auth.OAuthManager
	userAgent               string
	pendingState            []responses.ResponseInputItemUnionParam
	contextWindowTokenCount int
}

func NewOpenAICodexClient(cfg *ClientConfig) (*OpenAICodexClient, error) {
	if cfg.Provider != Provider(config.ProviderOpenAICodex) {
		return nil, fmt.Errorf("unsupported Codex OAuth provider: %s. %s", cfg.Provider, config.ConfigFixHint)
	}

	c := &OpenAICodexClient{
		model:                   cfg.Model,
		thinkingEffort:          cfg.ThinkingEffort,
		maxRetries:              retryCount(cfg.MaxRetries),
		client:                  openai.NewClient(option.WithBaseURL(openAICodexBaseURL), option.WithHTTPClient(newCodexHTTPClient())),
		contextWindowTokenCount: cfg.ContextWindowTokens,
		authManager:             auth.NewOAuthManager(nil),
		userAgent:               fmt.Sprintf("keen-agent (%s; %s)", runtime.GOOS, runtime.GOARCH),
	}
	c.responseStreamImpl = func(ctx context.Context, params responses.ResponseNewParams, opts ...option.RequestOption) responseStream {
		return &sdkResponseStream{stream: c.client.Responses.NewStreaming(ctx, params, opts...)}
	}
	return c, nil
}

func newCodexHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = false
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	return &http.Client{Transport: transport}
}

func (c *OpenAICodexClient) StreamChat(ctx context.Context, messages []Message, toolRegistry *tools.Registry, opts ...StreamOptions) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent)

	go func() {
		defer close(eventCh)

		instructions, input := codexInstructionsAndInput(messages)
		oneShot := streamOptions(opts).OneShot
		var injectedPending []responses.ResponseInputItemUnionParam
		if !oneShot {
			input, injectedPending = c.injectPendingState(input)
		}
		turnStartLen := len(input)
		responseTools := toOpenAIResponseTools(toolRegistry)

		for range maxToolTurns {
			reducedInput, reduction := reduceResponsesContextForRequest(c.contextWindowTokenCount, input)
			if !reduction.FitsBudget {
				slog.Debug("OpenAI Codex context still exceeds budget after reduction", "inputTokenCount", reduction.ReducedTokenCount, "removedToolResultCount", reduction.RemovedToolResults)
				c.pendingState = nil
				c.emitTerminalEvent(eventCh, input, turnStartLen, injectedPending, fmt.Errorf(contextWindowExceededError))
				return
			}
			input = reducedInput

			params := responses.ResponseNewParams{
				Model:        c.model,
				Instructions: param.NewOpt(instructions),
				Store:        param.NewOpt(false),
				Input: responses.ResponseNewParamsInputUnion{
					OfInputItemList: input,
				},
			}
			if c.thinkingEffort != "" {
				params.Reasoning = shared.ReasoningParam{
					Effort: reasoningEffortForLevel(c.thinkingEffort),
				}
			}
			if len(responseTools) > 0 {
				params.Tools = responseTools
			}

			completed, streamedContent, toolCalls, err := c.collectTurnWithRetry(ctx, params, eventCh)
			if err != nil {
				c.exitIncomplete(eventCh, input, turnStartLen, injectedPending, err, oneShot)
				return
			}
			if completed == nil {
				c.exitIncomplete(eventCh, input, turnStartLen, injectedPending, nil, oneShot)
				return
			}

			if completed.Usage.InputTokens > 0 || completed.Usage.OutputTokens > 0 {
				slog.Debug(
					"OpenAI Codex usage",
					"input_tokens", completed.Usage.InputTokens,
					"output_tokens", completed.Usage.OutputTokens,
					"total_tokens", completed.Usage.TotalTokens,
					"reasoning_tokens", completed.Usage.OutputTokensDetails.ReasoningTokens,
					"cached_tokens", completed.Usage.InputTokensDetails.CachedTokens,
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

		c.exitIncomplete(eventCh, input, turnStartLen, injectedPending, nil, oneShot)
	}()

	return eventCh, nil
}

func (c *OpenAICodexClient) Reset() {
	c.pendingState = nil
}

func codexInstructionsAndInput(messages []Message) (string, []responses.ResponseInputItemUnionParam) {
	instructions := make([]string, 0, 1)
	inputMessages := make([]Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == RoleSystem {
			content := strings.TrimSpace(FormatMessageForProvider(m))
			if content != "" {
				instructions = append(instructions, content)
			}
			continue
		}
		inputMessages = append(inputMessages, m)
	}
	return strings.Join(instructions, "\n\n"), toOpenAIResponseInput(inputMessages)
}

func (c *OpenAICodexClient) injectPendingState(input []responses.ResponseInputItemUnionParam) ([]responses.ResponseInputItemUnionParam, []responses.ResponseInputItemUnionParam) {
	if len(c.pendingState) == 0 {
		return input, nil
	}

	injectedPending := append([]responses.ResponseInputItemUnionParam(nil), c.pendingState...)

	slog.Debug("Injecting pending state", "pending_messages", len(c.pendingState), "total_messages", len(input))
	if prettyJSON, err := json.MarshalIndent(c.pendingState, "", "  "); err == nil {
		slog.Debug("Pending state contents:\n" + string(prettyJSON))
	}

	if len(input) > 0 {
		last := input[len(input)-1]
		input = append(input[:len(input)-1], injectedPending...)
		input = append(input, last)
	} else {
		input = append(input, injectedPending...)
	}
	c.pendingState = nil
	return input, injectedPending
}

func (c *OpenAICodexClient) exitIncomplete(eventCh chan<- StreamEvent, input []responses.ResponseInputItemUnionParam, turnStartLen int, injectedPending []responses.ResponseInputItemUnionParam, err error, oneShot bool) {
	if !oneShot {
		c.savePendingIfAccumulated(input, turnStartLen, injectedPending)
	}
	c.emitTerminalEvent(eventCh, input, turnStartLen, injectedPending, err)
}

func (c *OpenAICodexClient) savePendingIfAccumulated(input []responses.ResponseInputItemUnionParam, turnStartLen int, injectedPending []responses.ResponseInputItemUnionParam) {
	if len(injectedPending) == 0 && len(input) <= turnStartLen {
		return
	}

	newDelta := []responses.ResponseInputItemUnionParam(nil)
	if len(input) > turnStartLen {
		newDelta = input[turnStartLen:]
	}

	c.pendingState = make([]responses.ResponseInputItemUnionParam, 0, len(injectedPending)+len(newDelta))
	c.pendingState = append(c.pendingState, injectedPending...)
	c.pendingState = append(c.pendingState, newDelta...)
}

func (c *OpenAICodexClient) emitTerminalEvent(eventCh chan<- StreamEvent, input []responses.ResponseInputItemUnionParam, turnStartLen int, injectedPending []responses.ResponseInputItemUnionParam, err error) {
	if len(injectedPending) > 0 || len(input) > turnStartLen {
		eventCh <- StreamEvent{Type: StreamEventTypeIncomplete, Error: err}
	} else if err != nil {
		eventCh <- StreamEvent{Type: StreamEventTypeError, Error: err}
	} else {
		eventCh <- StreamEvent{Type: StreamEventTypeDone}
	}
}

func (c *OpenAICodexClient) collectTurnWithRetry(
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

func (c *OpenAICodexClient) collectTurn(
	ctx context.Context,
	params responses.ResponseNewParams,
	eventCh chan<- StreamEvent,
) (*responses.Response, string, []responses.ResponseFunctionToolCall, error) {
	opts, err := c.requestOptions(ctx)
	if err != nil {
		return nil, "", nil, err
	}
	stream := c.responseStreamImpl(ctx, params, opts...)
	var completed *responses.Response
	var streamedContent strings.Builder
	textProgress := make(map[string]int)
	toolCalls := make([]responses.ResponseFunctionToolCall, 0)
	seenToolCalls := make(map[string]bool)

	for stream.Next() {
		ev := stream.Current()

		switch ev.Type {
		case "response.output_text.delta":
			text := ev.Delta.OfString
			if text == "" {
				text = ev.AsResponseOutputTextDelta().Delta
			}
			emitCodexText(eventCh, &streamedContent, textProgress, codexTextProgressKey(ev), text)
		case "response.output_text.done":
			text := ev.Text
			if text == "" {
				text = ev.AsResponseOutputTextDone().Text
			}
			emitCodexTextDone(eventCh, &streamedContent, textProgress, codexTextProgressKey(ev), text)
		case "response.content_part.done":
			if ev.Part.Type == "output_text" {
				emitCodexTextDone(eventCh, &streamedContent, textProgress, codexTextProgressKey(ev), ev.Part.Text)
			}
		case "response.output_item.done":
			emitCodexOutputItemDone(eventCh, &streamedContent, textProgress, ev.Item)
			if ev.Item.Type == "function_call" {
				toolCalls = appendCodexToolCall(toolCalls, seenToolCalls, ev.Item.AsFunctionCall())
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
		return nil, streamedContent.String(), nil, formatCodexStreamError(err)
	}
	if completed == nil {
		return nil, streamedContent.String(), nil, nil
	}

	for _, item := range completed.Output {
		if item.Type != "function_call" {
			continue
		}
		toolCalls = appendCodexToolCall(toolCalls, seenToolCalls, item.AsFunctionCall())
	}

	return completed, streamedContent.String(), toolCalls, nil
}

func formatCodexStreamError(err error) error {
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("stream error: %w", err)
	}

	message := strings.TrimSpace(apiErr.Message)
	if message == "" {
		message = strings.TrimSpace(apiErr.RawJSON())
	}
	if message == "" {
		message = readCodexAPIErrorResponseBody(apiErr)
	}
	if message == "" {
		message = http.StatusText(apiErr.StatusCode)
	}

	parts := []string{fmt.Sprintf("HTTP %d", apiErr.StatusCode), message}
	if apiErr.Code != "" {
		parts = append(parts, "code="+apiErr.Code)
	}
	if apiErr.Param != "" {
		parts = append(parts, "param="+apiErr.Param)
	}
	return codexStreamError{
		message: fmt.Sprintf("OpenAI Codex API error: %s", strings.Join(parts, ": ")),
		err:     err,
	}
}

type codexStreamError struct {
	message string
	err     error
}

func (e codexStreamError) Error() string {
	return e.message
}

func (e codexStreamError) Unwrap() error {
	return e.err
}

func readCodexAPIErrorResponseBody(apiErr *openai.Error) string {
	if apiErr == nil || apiErr.Response == nil || apiErr.Response.Body == nil {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(apiErr.Response.Body, 4096))
	if err != nil {
		return ""
	}
	apiErr.Response.Body = io.NopCloser(bytes.NewBuffer(body))
	return strings.TrimSpace(string(body))
}

func codexTextProgressKey(ev responses.ResponseStreamEventUnion) string {
	return ev.ItemID + ":" + fmt.Sprint(ev.ContentIndex)
}

func emitCodexText(eventCh chan<- StreamEvent, streamedContent *strings.Builder, progress map[string]int, key string, text string) {
	if text == "" {
		return
	}
	streamedContent.WriteString(text)
	progress[key] += len(text)
	emitChunk(eventCh, text)
}

func emitCodexTextDone(eventCh chan<- StreamEvent, streamedContent *strings.Builder, progress map[string]int, key string, text string) {
	if text == "" {
		return
	}
	already := progress[key]
	if already >= len(text) {
		return
	}
	emitCodexText(eventCh, streamedContent, progress, key, text[already:])
}

func emitCodexOutputItemDone(eventCh chan<- StreamEvent, streamedContent *strings.Builder, progress map[string]int, item responses.ResponseOutputItemUnion) {
	if item.Type != "message" {
		return
	}
	for i, content := range item.Content {
		if content.Type != "output_text" {
			continue
		}
		emitCodexTextDone(eventCh, streamedContent, progress, fmt.Sprintf("%s:%d", item.ID, i), content.Text)
	}
}

func appendCodexToolCall(toolCalls []responses.ResponseFunctionToolCall, seen map[string]bool, toolCall responses.ResponseFunctionToolCall) []responses.ResponseFunctionToolCall {
	key := toolCall.CallID
	if key == "" {
		key = toolCall.ID
	}
	if key != "" {
		if seen[key] {
			return toolCalls
		}
		seen[key] = true
	}
	return append(toolCalls, toolCall)
}

func (c *OpenAICodexClient) executeTools(
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

func (c *OpenAICodexClient) requestOptions(ctx context.Context) ([]option.RequestOption, error) {
	cred, err := c.authManager.ValidAccessToken(ctx, auth.OpenAICodexProviderID)
	if err != nil {
		return nil, err
	}
	opts := []option.RequestOption{
		option.WithHeader("Authorization", "Bearer "+cred.AccessToken),
		option.WithHeader("originator", "keen-agent"),
		option.WithHeader("User-Agent", c.userAgent),
		option.WithHeaderDel("OpenAI-Organization"),
		option.WithHeaderDel("OpenAI-Project"),
	}
	if cred.AccountID != "" {
		opts = append(opts, option.WithHeader("ChatGPT-Account-Id", cred.AccountID))
	}
	return opts, nil
}
