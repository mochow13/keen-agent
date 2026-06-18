package repl

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	replcommands "github.com/mochow13/keen-agent/internal/cli/repl/commands"
	reploutput "github.com/mochow13/keen-agent/internal/cli/repl/output"
	replwidgets "github.com/mochow13/keen-agent/internal/cli/repl/widgets"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/providers"
)

func TestHandleLLMChunk(t *testing.T) {
	sh := NewStreamHandler(nil)
	sh.Start(make(<-chan llm.StreamEvent), "Loading...")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
	}

	newM, cmd := m.handleLLMChunk("hello")

	if !newM.showSpinner {
		t.Error("expected showSpinner to remain true after chunk")
	}
	if sh.GetResponse() != "hello" {
		t.Errorf("expected response 'hello', got '%s'", sh.GetResponse())
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
}

func TestContextStatus_UpdatesOnUsageEvent(t *testing.T) {
	m := newTestModel()
	m.ctx = &replContext{
		workingDir: "",
		cfg: &config.ResolvedConfig{
			Provider: "openai",
			Model:    "gpt-5.4",
		},
		registry: &providers.Registry{
			Providers: []providers.Provider{
				{
					ID: "openai",
					Models: []providers.Model{
						{ID: "gpt-5.4", ContextWindow: 2000},
					},
				},
			},
		},
	}
	m.appState = replappstate.New(nil, t.TempDir())
	m.refreshContextStatus()
	if m.contextStatus.Percent != 0 {
		t.Fatalf("expected 0%% initially, got %.2f", m.contextStatus.Percent)
	}

	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.showSpinner = true

	updatedAfterChunk, _ := m.handleLLMChunk("hello")
	if updatedAfterChunk.contextStatus.Percent != 0 {
		t.Fatalf("expected context percent to remain 0 during chunk, got %.2f", updatedAfterChunk.contextStatus.Percent)
	}

	updatedAfterUsage, _ := updatedAfterChunk.handleLLMUsage(&llm.TokenUsage{InputTokens: 1000})
	if updatedAfterUsage.contextStatus.Percent != 50.0 {
		t.Fatalf("expected 50%% after usage event, got %.2f", updatedAfterUsage.contextStatus.Percent)
	}
}

func TestHandleLLMDone(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")
	sh.HandleChunk("response line 1\nresponse line 2")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, t.TempDir()),
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	newM, cmd := m.handleLLMDone()

	if newM.showSpinner {
		t.Error("expected showSpinner to be false after done")
	}

	if len(m.appState.GetMessages()) != 1 {
		t.Errorf("expected 1 message in history, got %d", len(m.appState.GetMessages()))
	}
	if m.appState.GetMessages()[0].Role != llm.RoleAssistant {
		t.Errorf("expected assistant role, got %s", m.appState.GetMessages()[0].Role)
	}
	if m.appState.GetMessages()[0].Content != "response line 1\nresponse line 2" {
		t.Errorf("unexpected message content: %s", m.appState.GetMessages()[0].Content)
	}

	if len(newM.output.GetLines()) != 3 {
		t.Errorf("expected 3 output lines (2 content + 1 empty), got %d", len(newM.output.GetLines()))
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestHandleLLMError(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, t.TempDir()),
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	testErr := errors.New("stream failed")
	newM, cmd := m.handleLLMError(testErr)

	if newM.showSpinner {
		t.Error("expected showSpinner to be false after error")
	}

	if len(newM.output.GetLines()) != 2 {
		t.Errorf("expected 2 output lines (1 error + 1 empty), got %d", len(newM.output.GetLines()))
	}

	if !strings.Contains(newM.output.GetLines()[0], "stream failed") {
		t.Errorf("expected error message in output, got: %s", newM.output.GetLines()[0])
	}

	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestHandleKeyMsg_Enter(t *testing.T) {
	ta := textarea.New()
	ta.SetValue(replcommands.Help)
	m := replModel{
		textarea:      ta,
		width:         80,
		streamHandler: NewStreamHandler(nil),
		ctx:           &replContext{},
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !strings.Contains(newM.output.Join(), "Available Commands") {
		t.Error("expected help text in output after enter with /help")
	}
	if cmd != nil {
		t.Error("expected nil cmd for help command")
	}
}

func TestHandleKeyMsg_CtrlC_EmptyInputQuits(t *testing.T) {
	m := replModel{
		quitting: false,
	}

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if !newM.quitting {
		t.Error("expected quitting to be true after ctrl+c with empty input")
	}

	if cmd == nil {
		t.Fatal("expected tea.Quit cmd after ctrl+c with empty input")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHandleKeyMsg_CtrlC_WithInputClearsAndDoesNotQuit(t *testing.T) {
	ta := textarea.New()
	ta.SetValue("draft text")

	m := replModel{
		textarea: ta,
		quitting: false,
	}

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if newM.textarea.Value() != "" {
		t.Errorf("expected textarea to be cleared, got %q", newM.textarea.Value())
	}

	if newM.quitting {
		t.Error("expected quitting to remain false when ctrl+c clears input")
	}

	if cmd != nil {
		t.Error("expected nil cmd when ctrl+c clears input")
	}
}

func TestHandleKeyMsg_Esc_WithActiveStreamInterrupts(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.streamHandler.HandleChunk("partial response")
	m.showSpinner = true

	canceled := false
	m.streamCancel = func() {
		canceled = true
	}

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})

	if !canceled {
		t.Error("expected stream cancel function to be called on esc")
	}
	if newM.streamCancel != nil {
		t.Error("expected stream cancel function to be cleared after esc")
	}
	if newM.streamHandler.IsActive() {
		t.Error("expected stream handler to be inactive after esc interruption")
	}
	if newM.showSpinner {
		t.Error("expected spinner to be hidden after esc interruption")
	}
	if !strings.Contains(newM.output.Join(), "partial response") {
		t.Error("expected streamed partial content to be preserved on interruption")
	}
	if !strings.Contains(newM.output.Join(), "Interrupted") {
		t.Error("expected interrupted message in output")
	}
	if cmd != nil {
		t.Error("expected nil cmd for esc interruption")
	}
}

func TestHandleKeyMsg_Esc_WhenIdleNoOp(t *testing.T) {
	m := newTestModel()

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})

	if newM.quitting {
		t.Error("expected esc to not quit when no active stream")
	}
	if len(newM.output.GetLines()) != 0 {
		t.Error("expected no output when esc pressed without active stream")
	}
	if cmd != nil {
		t.Error("expected nil cmd when esc pressed without active stream")
	}
}

func TestHandleKeyMsg_Esc_CancelsCompactionBeforeSuggestions(t *testing.T) {
	m := newTestModel()
	m.isCompacting = true
	m.suggestion.Refresh("/c")
	cancelled := false
	m.compactionCancel = func() {
		cancelled = true
	}

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEsc})

	if !cancelled {
		t.Fatal("expected esc to cancel compaction")
	}
	if newM.compactionCancel != nil {
		t.Fatal("expected cancel func to be cleared after esc")
	}
	if !newM.suggestion.Visible() {
		t.Fatal("expected suggestions to remain untouched while compaction consumes input")
	}
	if cmd != nil {
		t.Fatal("expected no follow-up cmd")
	}
}

func TestHandleKeyMsg_IgnoresTypingDuringCompaction(t *testing.T) {
	m := newTestModel()
	m.isCompacting = true
	m.textarea.SetValue("draft")

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})

	if newM.textarea.Value() != "draft" {
		t.Fatalf("expected typing to be ignored during compaction, got %q", newM.textarea.Value())
	}
	if cmd != nil {
		t.Fatal("expected no cmd while compacting")
	}
}

func TestHandleKeyMsg_TabTogglesInputFocus(t *testing.T) {
	m := newTestModel()

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	if newM.textarea.Focused() {
		t.Fatal("expected tab to blur focused input")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd when blurring input")
	}

	newM, cmd = newM.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	if !newM.textarea.Focused() {
		t.Fatal("expected tab to focus blurred input")
	}
	if cmd == nil {
		t.Fatal("expected focus command when focusing input")
	}
}

func TestHandleKeyMsg_ShiftTabTogglesMode(t *testing.T) {
	m := newTestModel()

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Text: "shift+tab"})
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if newM.currentMode() != llm.ModePlan {
		t.Fatalf("expected plan mode, got %q", newM.currentMode())
	}
	if newM.appState.Mode() != llm.ModePlan {
		t.Fatalf("expected app state plan mode, got %q", newM.appState.Mode())
	}

	newM, _ = newM.handleKeyMsg(tea.KeyPressMsg{Text: "shift+tab"})
	if newM.currentMode() != llm.ModeBuild {
		t.Fatalf("expected build mode, got %q", newM.currentMode())
	}
}

func TestHandleKeyMsg_InputFocusUpDownDoesNotScrollViewportWhenHistoryExhausted(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("draft")
	offset := scrollViewportAwayFromBottom(t, &m)

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if got := newM.viewport.YOffset(); got != offset {
		t.Fatalf("expected input-focused up key not to scroll viewport, got offset %d want %d", got, offset)
	}
	if newM.textarea.Value() != "draft" {
		t.Fatalf("expected exhausted history to leave input unchanged, got %q", newM.textarea.Value())
	}

	newM, cmd = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if got := newM.viewport.YOffset(); got != offset {
		t.Fatalf("expected input-focused down key not to scroll viewport, got offset %d want %d", got, offset)
	}
}

func TestHandleKeyMsg_ViewportFocusUpDownScrolls(t *testing.T) {
	m := newTestModel()
	m.blurInput()
	m.viewport.SetHeight(6)
	for range 40 {
		m.output.AddLine("existing output")
	}
	m.updateViewportContent()
	m.viewport.GotoBottom()
	bottomOffset := m.viewport.YOffset()

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if newM.viewport.YOffset() >= bottomOffset {
		t.Fatalf("expected viewport-focused up key to scroll up from %d, got %d", bottomOffset, newM.viewport.YOffset())
	}
	upOffset := newM.viewport.YOffset()

	newM, cmd = newM.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if newM.viewport.YOffset() <= upOffset {
		t.Fatalf("expected viewport-focused down key to scroll down from %d, got %d", upOffset, newM.viewport.YOffset())
	}
}

func TestHandleKeyMsg_ViewportFocusTypingFocusesInputAndTypes(t *testing.T) {
	m := newTestModel()
	m.blurInput()

	newM, cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if !newM.textarea.Focused() {
		t.Fatal("expected typing to focus input")
	}
	if newM.textarea.Value() != "x" {
		t.Fatalf("expected typed character in input, got %q", newM.textarea.Value())
	}
	if cmd == nil {
		t.Fatal("expected focus/update command")
	}
}

func TestHandleKeyMsg_CtrlJ(t *testing.T) {
	ta := textarea.New()
	ta.Focus()
	ta.KeyMap.InsertNewline.SetKeys("ctrl+j")
	ta.KeyMap.InsertNewline.SetEnabled(true)
	ta.SetValue("line 1")
	ta.CursorEnd()
	m := replModel{
		textarea: ta,
		width:    80,
	}

	newM, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})

	if !strings.Contains(newM.textarea.Value(), "\n") {
		t.Error("expected newline in textarea after ctrl+j")
	}
}

func TestHandleKeyMsg_ModelSelectionMode(t *testing.T) {
	m := replModel{
		width:          80,
		modelSelection: &replwidgets.Model{},
	}

	newM, _ := m.handleKeyMsg(tea.KeyPressMsg{Code: 'a', Text: "a"})

	if newM.modelSelection == nil {
		t.Error("expected modelSelection to remain set")
	}
}

func TestUpdateNormalMode_ModelSelectionPasteGoesToAPIKeyInput(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("existing prompt")
	m.modelSelection = &replwidgets.Model{
		Step: replwidgets.StepAPIKey,
	}

	newM, _ := m.updateNormalMode(tea.PasteMsg{Content: "sk-test-123"})

	if newM.modelSelection.APIKeyInput != "sk-test-123" {
		t.Fatalf("expected pasted API key to go to model selection, got %q", newM.modelSelection.APIKeyInput)
	}
	if newM.textarea.Value() != "existing prompt" {
		t.Fatalf("expected textarea to remain unchanged, got %q", newM.textarea.Value())
	}
}

func TestHandleLLMChunk_MultipleCalls(t *testing.T) {
	sh := NewStreamHandler(nil)
	sh.Start(make(<-chan llm.StreamEvent), "Loading...")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
	}

	m, _ = m.handleLLMChunk("Hello")
	m, _ = m.handleLLMChunk(" ")
	m, _ = m.handleLLMChunk("World")

	if sh.GetResponse() != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", sh.GetResponse())
	}

	if !m.showSpinner {
		t.Error("showSpinner should remain true during streaming")
	}
}

func TestHandleLLMDone_EmptyResponse(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, t.TempDir()),
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	newM, _ := m.handleLLMDone()

	if len(m.appState.GetMessages()) != 1 {
		t.Errorf("expected 1 message, got %d", len(m.appState.GetMessages()))
	}

	if len(newM.output.GetLines()) != 1 {
		t.Errorf("expected 1 line (trailing empty spacer), got %d", len(newM.output.GetLines()))
	}
}

func TestHandleLLMError_ResetsHandler(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")
	sh.HandleChunk("partial content")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, t.TempDir()),
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	newM, _ := m.handleLLMError(errors.New("fail"))

	if sh.IsActive() {
		t.Error("handler should not be active after error")
	}
	if sh.HasContent() {
		t.Error("handler should not have content after error")
	}

	_ = newM
}

func TestHandleLLMError_MaterializesMessageAndTurnMemory(t *testing.T) {
	workingDir := t.TempDir()
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")
	sh.HandleChunk("partial response")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, workingDir),
		output:        reploutput.NewOutputBuilder(80, ""),
	}
	m.startAssistantTurnMemory()
	m.recordToolMemory(&llm.ToolCall{
		Name:   "write_file",
		Output: map[string]any{"path": workingDir + "/foo.go"},
	})

	updated, _ := m.handleLLMError(errors.New("rate limit"))

	messages := updated.appState.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 materialized assistant message, got %d", len(messages))
	}
	msg := messages[0]
	if msg.Role != llm.RoleAssistant {
		t.Errorf("expected assistant role, got %s", msg.Role)
	}
	if msg.Content != "partial response" {
		t.Errorf("expected partial response content, got %q", msg.Content)
	}
	if msg.TurnMemory == nil {
		t.Fatal("expected TurnMemory to be preserved, got nil")
	}
	if len(msg.TurnMemory.FilesChanged) != 1 {
		t.Fatalf("expected 1 changed file, got %#v", msg.TurnMemory.FilesChanged)
	}
}

func TestHandleLLMError_ContextCanceled_DoesNotAddErrorLine(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")
	sh.HandleChunk("partial content")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, t.TempDir()),
		output:        reploutput.NewOutputBuilder(80, ""),
		streamCancel:  func() {},
	}

	newM, cmd := m.handleLLMError(context.Canceled)

	if len(newM.output.GetLines()) != 1 {
		t.Fatalf("expected only pending transcript line, got %d", len(newM.output.GetLines()))
	}
	if strings.Contains(newM.output.Join(), "context canceled") {
		t.Error("expected cancellation to not render an error line")
	}
	if newM.streamCancel != nil {
		t.Error("expected stream cancel function to be cleared")
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestHandleToolStart(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	toolCall := &llm.ToolCall{
		Name:  "test_tool",
		Input: map[string]any{"arg1": "value1"},
	}

	newM, cmd := m.handleToolStart(toolCall)

	if !newM.showSpinner {
		t.Error("expected showSpinner to remain true after tool start")
	}

	if len(newM.output.GetLines()) != 0 {
		t.Errorf("expected no persisted output lines for tool start, got %d", len(newM.output.GetLines()))
	}

	if cmd == nil {
		t.Error("expected non-nil cmd from handleToolStart")
	}

	if len(sh.segments) != 1 {
		t.Errorf("expected 1 stream segment in handler, got %d", len(sh.segments))
	}

	if sh.segments[0].kind != segmentToolStart {
		t.Errorf("expected first segment kind %q, got %q", segmentToolStart, sh.segments[0].kind)
	}
}

func TestHandleToolStart_BashKeepsSpinnerActive(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	toolCall := &llm.ToolCall{
		Name:  "bash",
		Input: map[string]any{"command": "npm test", "summary": "running tests"},
	}

	newM, cmd := m.handleToolStart(toolCall)

	if !newM.showSpinner {
		t.Error("expected showSpinner to remain true for running bash")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from handleToolStart")
	}
	if len(sh.segments) != 1 || sh.segments[0].kind != segmentBash {
		t.Fatalf("expected a bash segment to be added")
	}
}

func TestHandleToolEnd(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := replModel{
		streamHandler: sh,
		width:         80,
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	toolCall := &llm.ToolCall{
		Name:     "test_tool",
		Input:    map[string]any{"arg1": "value1"},
		Output:   "tool result",
		Duration: 1500000000,
	}

	newM, cmd := m.handleToolEnd(toolCall)

	if len(newM.output.GetLines()) != 0 {
		t.Errorf("expected no persisted output lines for tool end, got %d", len(newM.output.GetLines()))
	}

	if cmd == nil {
		t.Error("expected non-nil cmd from handleToolEnd")
	}

	if len(sh.segments) != 1 {
		t.Errorf("expected 1 stream segment in handler, got %d", len(sh.segments))
	}

	if sh.segments[0].kind != segmentToolEnd {
		t.Errorf("expected first segment kind %q, got %q", segmentToolEnd, sh.segments[0].kind)
	}
}

func TestHandleToolEnd_WithError(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := replModel{
		streamHandler: sh,
		width:         80,
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	toolCall := &llm.ToolCall{
		Name:  "test_tool",
		Input: map[string]any{"arg1": "value1"},
		Error: "connection failed",
	}

	newM, cmd := m.handleToolEnd(toolCall)

	if len(newM.output.GetLines()) != 0 {
		t.Errorf("expected no persisted output lines for tool end, got %d", len(newM.output.GetLines()))
	}

	if cmd == nil {
		t.Error("expected non-nil cmd from handleToolEnd")
	}

	if len(sh.segments) != 1 {
		t.Errorf("expected 1 stream segment in handler, got %d", len(sh.segments))
	}

	if sh.segments[0].kind != segmentToolEnd {
		t.Errorf("expected first segment kind %q, got %q", segmentToolEnd, sh.segments[0].kind)
	}

	if sh.segments[0].toolCall == nil || sh.segments[0].toolCall.Error != "connection failed" {
		t.Errorf("expected tool end segment with error details")
	}
}

func TestHandleLLMStreamMsg_ToolEnd_ReturnsSpinnerTick(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	m := newTestModel()
	m.streamHandler = sh
	m.showSpinner = true

	toolCall := &llm.ToolCall{
		Name:   "test_tool",
		Input:  map[string]any{"arg1": "value1"},
		Output: "tool result",
	}

	updated, cmd, handled := m.handleLLMStreamMsg(llmToolEndMsg{toolCall: toolCall})

	if !handled {
		t.Error("expected tool end msg to be handled")
	}

	if !updated.showSpinner {
		t.Error("expected showSpinner to remain true after tool end")
	}

	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleBtwStreamMsg_Chunk(t *testing.T) {
	btwSh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	btwSh.Start(eventCh, "Loading...")

	m := newTestModel()
	m.btwStreamHandler = btwSh

	updated, cmd, handled := m.handleBtwStreamMsg(btwChunkMsg("hello"))

	if !handled {
		t.Fatal("expected btw chunk msg to be handled")
	}
	if btwSh.GetResponse() != "hello" {
		t.Fatalf("expected btw response 'hello', got %q", btwSh.GetResponse())
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd for btw chunk")
	}
	_ = updated
}

func TestHandleBtwStreamMsg_Done(t *testing.T) {
	btwSh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	btwSh.Start(eventCh, "Loading...")
	btwSh.HandleChunk("answer text")

	m := newTestModel()
	m.btwStreamHandler = btwSh
	m.btwShowSpinner = true
	m.btwQuestion = "what?"

	updated, cmd, handled := m.handleBtwStreamMsg(btwDoneMsg{})

	if !handled {
		t.Fatal("expected btw done msg to be handled")
	}
	if updated.btwShowSpinner {
		t.Fatal("expected btw spinner to stop after done")
	}
	if updated.btwLines == nil {
		t.Fatal("expected btwLines to be set after done")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd after btw done")
	}
}

func TestHandleBtwStreamMsg_Error(t *testing.T) {
	btwSh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	btwSh.Start(eventCh, "Loading...")

	m := newTestModel()
	m.btwStreamHandler = btwSh
	m.btwShowSpinner = true
	m.btwQuestion = "question"

	updated, cmd, handled := m.handleBtwStreamMsg(btwErrorMsg{err: errors.New("oops")})

	if !handled {
		t.Fatal("expected btw error msg to be handled")
	}
	if updated.btwShowSpinner {
		t.Fatal("expected btw spinner to stop after error")
	}
	if updated.btwLines == nil {
		t.Fatal("expected btwLines to be set after error")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd after btw error")
	}
}

func TestHandleBtwStreamMsg_InactiveHandlerSwallowsMessages(t *testing.T) {
	btwSh := NewStreamHandler(nil)

	m := newTestModel()
	m.btwStreamHandler = btwSh

	_, _, handled := m.handleBtwStreamMsg(btwChunkMsg("orphan"))

	if !handled {
		t.Fatal("expected stale btw chunk to be swallowed")
	}
}
