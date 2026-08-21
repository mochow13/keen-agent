package repl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	replcommands "github.com/mochow13/keen-agent/internal/cli/repl/commands"
	reploutput "github.com/mochow13/keen-agent/internal/cli/repl/output"
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	repltheme "github.com/mochow13/keen-agent/internal/cli/repl/theme"
	replwidgets "github.com/mochow13/keen-agent/internal/cli/repl/widgets"
	"github.com/mochow13/keen-agent/internal/llm"
)

const (
	keyEnter     = "enter"
	keyCtrlC     = "ctrl+c"
	keyCtrlD     = "ctrl+d"
	keyEsc       = "esc"
	keyTab       = "tab"
	keyUp        = "up"
	keyDown      = "down"
	keyPageUp    = "pgup"
	keyPageDown  = "pgdown"
	keyHome      = "home"
	keyEnd       = "end"
	keyShiftUp   = "shift+up"
	keyShiftDown = "shift+down"
)

func (m *replModel) handleLLMUsage(usage *llm.TokenUsage) (replModel, tea.Cmd) {
	if m.appState != nil && usage != nil {
		m.appState.SetLastUsage(usage)
		m.contextStatus.AddUsage(usage)
		m.refreshContextStatus()
	}
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleLLMChunk(chunk string) (replModel, tea.Cmd) {
	m.streamHandler.HandleChunk(chunk)
	return *m, tea.Batch(m.afterStreamUpdate(), m.waitForAsyncEvent())
}

func (m *replModel) handleLLMReasoningChunk(chunk string) (replModel, tea.Cmd) {
	m.streamHandler.HandleReasoningChunk(chunk)
	return *m, tea.Batch(m.afterStreamUpdate(), m.waitForAsyncEvent())
}

func (m *replModel) handleLLMDone() (replModel, tea.Cmd) {
	m.flushStreamRender()
	if m.isCompacting {
		return m.handleCompactionDone()
	}
	segments := cloneStreamSegments(m.streamHandler.segments)
	m.recordHistoricalToolActivity(segments)
	m.stopLoading()
	m.clearStreamCancel()
	m.adjustTextareaHeight()
	responseLines, response := m.streamHandler.HandleDone()
	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    response,
		TurnMemory: m.consumeTurnMemory(),
	}
	m.appState.AppendMessage(assistantMessage)
	if err := m.sessions.appendAssistantTurn(segments, assistantMessage, false, ""); err != nil {
		m.handleSessionPersistenceError(err)
	}
	m.refreshContextStatus()
	for _, line := range responseLines {
		m.output.AddLine(line)
	}
	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return m.drainQueuedInput()
}

func (m *replModel) handleLLMIncomplete(err error) (replModel, tea.Cmd) {
	m.flushStreamRender()
	segments := cloneStreamSegments(m.streamHandler.segments)
	m.recordHistoricalToolActivity(segments)
	partialResponse := m.streamHandler.GetResponse()
	m.stopLoading()
	m.clearStreamCancel()
	turnMemory := m.consumeTurnMemory()
	m.adjustTextareaHeight()
	pendingLines, errMsg := m.streamHandler.HandleError(err)
	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    partialResponse,
		TurnMemory: turnMemory,
	}
	if persistErr := m.sessions.appendAssistantTurn(segments, assistantMessage, false, errMsg); persistErr != nil {
		m.handleSessionPersistenceError(persistErr)
	}
	for _, line := range pendingLines {
		m.output.AddLine(line)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		m.output.AddError(errMsg, repltheme.ErrorStyle)
	}
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return m.drainQueuedInput()
}

func (m *replModel) handleLLMError(err error) (replModel, tea.Cmd) {
	m.flushStreamRender()
	if m.isCompacting {
		return m.handleCompactionError(err)
	}
	segments := cloneStreamSegments(m.streamHandler.segments)
	m.recordHistoricalToolActivity(segments)
	partialResponse := m.streamHandler.GetResponse()
	m.stopLoading()
	m.clearStreamCancel()
	turnMemory := m.consumeTurnMemory()
	m.adjustTextareaHeight()
	pendingLines, errMsg := m.streamHandler.HandleError(err)
	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    partialResponse,
		TurnMemory: turnMemory,
	}
	if partialResponse != "" || (turnMemory != nil && !turnMemory.IsEmpty()) {
		m.appState.AppendMessage(assistantMessage)
		if persistErr := m.sessions.appendAssistantTurn(segments, assistantMessage, false, errMsg); persistErr != nil {
			m.handleSessionPersistenceError(persistErr)
		}
	}
	for _, line := range pendingLines {
		m.output.AddLine(line)
	}
	if errors.Is(err, context.Canceled) {
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return *m, nil
	}
	m.output.AddError(errMsg, repltheme.ErrorStyle)
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, nil
}

func (m *replModel) handleLLMRetry(err error, attempt int) (replModel, tea.Cmd) {
	m.flushStreamRender()
	m.streamHandler.RewindForRetry()
	m.rebuildTurnMemoryFromSegments(m.streamHandler.segments)
	m.loadingText = fmt.Sprintf("Retrying (attempt %d)...", attempt)
	m.streamHandler.SetLoadingText(m.loadingText)
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleAutoCompactionApplied(event *llm.AutoCompactionEvent) (replModel, tea.Cmd) {
	if event == nil || len(event.Replacement) == 0 {
		return *m, m.waitForAsyncEvent()
	}
	m.appState.ReplaceMessages(replappstate.WithoutSystemMessages(event.Replacement))
	m.refreshContextStatus()
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleCompactionDone() (replModel, tea.Cmd) {
	m.flushStreamRender()
	segments := cloneStreamSegments(m.streamHandler.segments)
	responseLines, summary := m.streamHandler.HandleDone()
	m.isCompacting = false
	m.stopLoading()
	m.compactionCancel = nil
	m.clearStreamCancel()
	if err := m.appState.ApplyCompaction(summary); err != nil {
		return m.handleCompactionError(err)
	}
	m.refreshContextStatus()
	for _, line := range responseLines {
		m.output.AddLine(line)
	}
	if len(responseLines) > 0 {
		m.output.AddEmptyLine()
	}
	if err := m.sessions.appendCompaction(segments, m.appState.GetMessages(), "Context compacted."); err != nil {
		m.handleSessionPersistenceError(err)
	}
	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return m.drainQueuedInput()
}

func (m *replModel) handleCompactionError(err error) (replModel, tea.Cmd) {
	m.flushStreamRender()
	if m.streamHandler != nil && m.streamHandler.IsActive() {
		responseLines, _ := m.streamHandler.HandleError(err)
		for _, line := range responseLines {
			m.output.AddLine(line)
		}
		if len(responseLines) > 0 {
			m.output.AddEmptyLine()
		}
	}
	m.isCompacting = false
	m.stopLoading()
	m.compactionCancel = nil
	m.clearStreamCancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			reploutput.AddCompactionCancelledStatus(m.output, "Compaction cancelled.")
		} else {
			status := "Compaction failed: " + err.Error()
			reploutput.AddCompactionErrorStatus(m.output, status)
		}
	}
	m.adjustTextareaHeight()
	m.refreshContextStatus()
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, nil
}

func (m *replModel) handleToolStart(toolCall *llm.ToolCall) (replModel, tea.Cmd) {
	m.flushStreamRender()
	if toolCall.Name == "bash" {
		command, _ := toolCall.Input["command"].(string)
		summary, _ := toolCall.Input["summary"].(string)
		m.streamHandler.HandleBashStart(command, summary)
	} else {
		m.streamHandler.HandleToolStart(toolCall)
	}
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleToolEnd(toolCall *llm.ToolCall) (replModel, tea.Cmd) {
	m.flushStreamRender()
	if toolCall.Name == "bash" {
		m.streamHandler.HandleBashEnd(toolCall)
	} else {
		m.streamHandler.HandleToolEnd(toolCall)
	}
	m.loadingText = nextLoadingText()
	m.streamHandler.SetLoadingText(m.loadingText)
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, m.waitForAsyncEvent()
}

// extractAtToken scans backwards from cursorPos in input to find a @<token>.
// The @ must be at the start of input or preceded by a space.
// Returns the token text (without @), the start index of @, and found=true if valid.
func extractAtToken(input string, cursorPos int) (token string, startIdx int, found bool) {
	if cursorPos <= 0 || cursorPos > len(input) {
		return "", 0, false
	}
	sub := input[:cursorPos]
	atIdx := strings.LastIndex(sub, "@")
	if atIdx < 0 {
		return "", 0, false
	}
	if atIdx > 0 && input[atIdx-1] != ' ' {
		return "", 0, false
	}
	tok := sub[atIdx+1:]
	if len(tok) == 0 {
		return "", 0, false
	}
	if strings.ContainsRune(tok, ' ') {
		return "", 0, false
	}
	return tok, atIdx, true
}

func (m *replModel) handleFileModeSelection() (replModel, tea.Cmd) {
	var item *replwidgets.SuggestionItem
	if cur := m.suggestion.Current(); cur != nil {
		item = cur
	} else if first := m.suggestion.First(); first != nil {
		item = first
	}
	if item != nil {
		val := m.textarea.Value()
		linesBefore := strings.Split(val, "\n")
		cursorByte := 0
		for i, ln := range linesBefore {
			if i == m.textarea.Line() {
				cursorByte += m.textarea.Column()
				break
			}
			cursorByte += len(ln) + 1
		}
		if _, atIdx, found := extractAtToken(val, cursorByte); found {
			replacement := "@" + item.Name + " "
			newVal := val[:atIdx] + replacement + val[cursorByte:]
			m.textarea.SetValue(newVal)
			m.textarea.MoveToEnd()
		}
	}
	m.suggestion.Hide()
	m.adjustTextareaHeight()
	return *m, nil
}

func (m *replModel) handleSuggestionKeyMsg(keyMsg tea.KeyPressMsg) (bool, replModel, tea.Cmd) {
	switch keyMsg.String() {
	case keyEnter, keyTab:
		if m.suggestion.IsFileMode() {
			result, cmd := m.handleFileModeSelection()
			return true, result, cmd
		}
		if keyMsg.String() == keyEnter && !m.suggestion.IsModelMode() && (m.textarea.Value() == replcommands.Model || (m.suggestion.Current() != nil && m.suggestion.Current().Name == replcommands.Model)) {
			m.textarea.SetValue(replcommands.Model)
			m.suggestion.Hide()
			m.refreshSuggestions(m.textarea.Value())
			m.adjustTextareaHeight()
			return true, *m, nil
		}
		if keyMsg.String() == keyEnter && m.textarea.Value() == replcommands.Model && m.suggestion.IsModelMode() && m.suggestion.IsFirstSelected() {
			m.suggestion.Hide()
			m.adjustTextareaHeight()
			result, cmd := m.handleEnterKey()
			return true, result, cmd
		}
		if cur := m.suggestion.Current(); cur != nil {
			m.textarea.SetValue(suggestionValue(cur))
		} else if first := m.suggestion.First(); first != nil {
			m.textarea.SetValue(suggestionValue(first))
		}
		if keyMsg.String() == keyEnter {
			m.suggestion.Hide()
		} else {
			m.refreshSuggestions(m.textarea.Value())
		}
		m.adjustTextareaHeight()
		return true, *m, nil
	case keyUp, keyShiftUp:
		m.suggestion.MoveUp()
		return true, *m, nil
	case keyDown, keyShiftDown:
		m.suggestion.MoveDown()
		return true, *m, nil
	case keyEsc:
		if m.streamHandler == nil || !m.streamHandler.IsActive() {
			m.suggestion.Refresh("")
			m.adjustTextareaHeight()
			return true, *m, nil
		}
	}
	return false, *m, nil
}

func suggestionValue(item *replwidgets.SuggestionItem) string {
	if item.Value != "" {
		return item.Value
	}
	return item.Name
}

func (m *replModel) handleKeyMsg(msg tea.Msg) (replModel, tea.Cmd) {
	m.flushStreamRender()
	if m.sessionPicker != nil {
		return m.handleSessionPickerKeyMsg(msg)
	}

	if m.modelSelection != nil {
		var cmd tea.Cmd
		m.modelSelection, cmd = m.modelSelection.Update(msg)
		m.updateViewportContent()
		return *m, cmd
	}

	if m.adversary.modelSelection != nil {
		var cmd tea.Cmd
		m.adversary.modelSelection, cmd = m.adversary.modelSelection.Update(msg)
		m.updateViewportContent()
		return *m, cmd
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}

	if m.isCompacting {
		if keyMsg.String() == keyEsc && m.compactionCancel != nil {
			m.compactionCancel()
			m.compactionCancel = nil
		}
		return *m, nil
	}

	if m.streamHandler != nil && m.streamHandler.HasPendingPermission() {
		switch keyMsg.String() {
		case "up", "down", keyEnter, keyEsc:
			return m.handlePermissionKeyMsg(keyMsg)
		}
	}

	if m.suggestion.Visible() {
		if handled, result, cmd := m.handleSuggestionKeyMsg(keyMsg); handled {
			return result, cmd
		}
	} else if keyMsg.String() == "shift+tab" {
		m.toggleMode()
		return *m, nil
	} else if keyMsg.String() == keyTab {
		return *m, m.toggleInputFocus()
	}

	if !m.textarea.Focused() {
		if handled := m.handleViewportFocusKeyMsg(keyMsg); handled {
			return *m, nil
		}
		if keyMsg.Text != "" {
			cmd := m.focusInput()
			var textCmd tea.Cmd
			m.textarea, textCmd = m.textarea.Update(keyMsg)
			input := m.textarea.Value()
			m.refreshSuggestions(input)
			m.adjustTextareaHeight()
			return *m, tea.Batch(cmd, textCmd)
		}
	}

	switch keyMsg.String() {
	case keyEnter:
		return m.handleEnterKey()
	case keyCtrlC, keyCtrlD:
		if m.adversary.streamHandler != nil && m.adversary.streamHandler.IsActive() {
			m.cancelAdversaryStream()
			m.updateViewportContent()
			m.scrollToBottomIfFollowing()
			return *m, nil
		}
		if m.btwStreamHandler != nil && m.btwStreamHandler.IsActive() {
			m.cancelBtwStream()
			m.updateViewportContent()
			m.scrollToBottomIfFollowing()
			return *m, nil
		}
		if m.bang.active {
			m.cancelBangCommand()
			return *m, nil
		}
		if m.streamHandler != nil && m.streamHandler.IsActive() {
			m.interruptStream(interruptedPromptText)
			return *m, nil
		}
		if len(m.queuedInputs) > 0 {
			m.queuedInputs = nil
			m.updateViewportContent()
			m.adjustTextareaHeight()
			return *m, m.showNotification("Queue cleared")
		}
		if m.textarea.Value() != "" {
			m.textarea.Reset()
			m.adjustTextareaHeight()
			return *m, nil
		}
		m.quitting = true
		_ = m.history.Flush()
		return *m, tea.Quit
	case keyEsc:
		if m.adversary.streamHandler != nil && m.adversary.streamHandler.IsActive() {
			m.cancelAdversaryStream()
			m.updateViewportContent()
			m.scrollToBottomIfFollowing()
			return *m, nil
		}
		if m.btwStreamHandler != nil && m.btwStreamHandler.IsActive() {
			m.cancelBtwStream()
			m.updateViewportContent()
			m.scrollToBottomIfFollowing()
			return *m, nil
		}
		if m.bang.active {
			m.cancelBangCommand()
			return *m, nil
		}
		if m.streamHandler != nil && m.streamHandler.IsActive() {
			m.interruptStream(interruptedPromptText)
		} else if len(m.queuedInputs) > 0 {
			m.queuedInputs = nil
			m.updateViewportContent()
			m.adjustTextareaHeight()
			return *m, m.showNotification("Queue cleared")
		}
		return *m, nil
	case keyUp, keyShiftUp:
		if m.isAtTopOfInput() {
			if !m.history.IsNavigating() && m.textarea.Column() > 0 {
				m.textarea.MoveToBegin()
				return *m, nil
			}
			if val, ok := m.history.NavigateUp(m.textarea.Value()); ok {
				m.textarea.SetValue(val)
				m.textarea.MoveToEnd()
				m.adjustTextareaHeight()
			}
			return *m, nil
		}
	case keyDown, keyShiftDown:
		if m.isAtBottomOfInput() {
			if val, ok := m.history.NavigateDown(); ok {
				m.textarea.SetValue(val)
				m.textarea.MoveToEnd()
				m.adjustTextareaHeight()
			}
			return *m, nil
		}
	case keyPageUp:
		m.viewport.HalfPageUp()
		m.userScrolled = !m.viewport.AtBottom()
		return *m, nil
	case keyPageDown:
		m.viewport.HalfPageDown()
		m.userScrolled = !m.viewport.AtBottom()
		return *m, nil
	case keyHome:
		m.viewport.GotoTop()
		m.userScrolled = true
		return *m, nil
	case keyEnd:
		m.viewport.GotoBottom()
		m.userScrolled = false
		return *m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(keyMsg)
	input := m.textarea.Value()
	m.refreshSuggestions(input)
	m.adjustTextareaHeight()
	return *m, cmd
}

func (m *replModel) refreshSuggestions(input string) {
	if input == "" {
		m.suggestion.Hide()
		return
	}
	if m.refreshFileSuggestions(input) {
		return
	}
	if strings.HasPrefix(input, replcommands.Model) && (input == replcommands.Model || strings.HasPrefix(input, replcommands.Model+" ")) {
		m.suggestion.RefreshModels(input, m.modelPairs())
		return
	}
	if strings.HasPrefix(input, "/") {
		m.suggestion.RefreshWithSkillsAndHelpers(input, m.skillSuggestions(), m.btwEnabled(), m.adversaryEnabled())
	}
}

func (m *replModel) modelPairs() []string {
	if m.ctx == nil || m.ctx.registry == nil {
		return nil
	}
	pairs := make([]string, 0)
	for _, provider := range m.ctx.registry.Providers {
		for _, model := range provider.Models {
			pairs = append(pairs, provider.ID+"/"+model.ID)
		}
	}
	return pairs
}

func (m *replModel) skillSuggestions() []replwidgets.SuggestionItem {
	skillList := m.appState.SkillSuggestions()
	items := make([]replwidgets.SuggestionItem, 0, len(skillList))
	for _, skill := range skillList {
		items = append(items, replwidgets.SuggestionItem{Name: "/" + skill.Name, Description: skill.Description})
	}
	return items
}

func (m *replModel) refreshFileSuggestions(input string) bool {
	if m.fileSearcher == nil {
		m.suggestion.Hide()
		return false
	}
	linesBefore := strings.Split(input, "\n")
	cursorByte := 0
	for i, ln := range linesBefore {
		if i == m.textarea.Line() {
			cursorByte += m.textarea.Column()
			break
		}
		cursorByte += len(ln) + 1
	}
	if tok, _, found := extractAtToken(input, cursorByte); found {
		paths := m.fileSearcher.Search(tok, 10)
		m.suggestion.RefreshFiles(paths)
		return true
	}
	m.suggestion.Hide()
	return false
}

func (m *replModel) interruptStream(message string) {
	m.flushStreamRender()
	if m.streamCancel != nil {
		m.streamCancel()
		m.clearStreamCancel()
	}

	m.stopLoading()

	segments := cloneStreamSegments(m.streamHandler.segments)
	m.recordHistoricalToolActivity(segments)
	partialResponse := m.streamHandler.GetResponse()
	turnMemory := m.consumeTurnMemory()

	for _, line := range m.streamHandler.HandleInterrupt() {
		m.output.AddLine(line)
	}
	m.output.AddStyledLine("\n  "+message, repltheme.InterruptedStyle)
	m.output.AddEmptyLine()

	assistantMessage := llm.Message{
		Role:       llm.RoleAssistant,
		Content:    partialResponse,
		TurnMemory: turnMemory,
	}
	if persistErr := m.sessions.appendAssistantTurn(segments, assistantMessage, true, ""); persistErr != nil {
		m.handleSessionPersistenceError(persistErr)
	}

	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
}

func (m *replModel) handleSessionPickerKeyMsg(msg tea.Msg) (replModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || m.sessionPicker == nil {
		return *m, nil
	}

	switch keyMsg.String() {
	case keyUp, keyShiftUp:
		m.sessionPicker.Move(-1)
		m.updateViewportContent()
	case keyDown, keyShiftDown:
		m.sessionPicker.Move(1)
		m.updateViewportContent()
	case keyEnter:
		selected := m.sessionPicker.Current()
		if selected == nil {
			return *m, nil
		}
		loaded, err := m.sessions.load(*selected)
		if err != nil {
			m.sessionPicker = nil
			m.handleSessionPersistenceError(err)
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil
		}
		m.replayLoadedSession(loaded)
	case keyEsc:
		m.sessionPicker = nil
		m.updateViewportContent()
		m.viewport.GotoBottom()
	}

	return *m, nil
}

func (m *replModel) handlePermissionKeyMsg(msg tea.KeyPressMsg) (replModel, tea.Cmd) {
	switch msg.String() {
	case "up":
		m.streamHandler.MovePendingCursor(-1)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
	case "down":
		m.streamHandler.MovePendingCursor(1)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
	case keyEnter:
		req := m.streamHandler.GetPendingPermissionRequest()
		if req == nil {
			return *m, nil
		}
		choice := m.streamHandler.GetPendingChoice()
		if choice == replpermissions.ChoiceAskWhatToDo {
			m.streamHandler.ResolvePendingPermission(replpermissions.StatusRedirected)
			m.permissionRequester.SendResponse(replpermissions.ChoiceDeny, req.ToolName)
			m.interruptStream(interruptedPromptText)
			m.updateViewportContent()
			m.scrollToBottomIfFollowing()
			return *m, nil
		}
		var status replpermissions.Status
		switch choice {
		case replpermissions.ChoiceAllow:
			status = replpermissions.StatusAllowed
		case replpermissions.ChoiceAllowSession:
			status = replpermissions.StatusAllowedSession
		case replpermissions.ChoiceDeny:
			status = replpermissions.StatusDenied
		}
		m.streamHandler.ResolvePendingPermission(status)
		m.permissionRequester.SendResponse(choice, req.ToolName)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
	case keyEsc:
		req := m.streamHandler.GetPendingPermissionRequest()
		if req == nil {
			return *m, nil
		}
		m.streamHandler.ResolvePendingPermission(replpermissions.StatusDenied)
		m.permissionRequester.SendResponse(replpermissions.ChoiceDeny, req.ToolName)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
	}
	return *m, nil
}

func (m replModel) handleLLMStreamMsg(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if updated, cmd, handled := m.handleBtwStreamMsg(msg); handled {
		return updated, cmd, true
	}

	if updated, cmd, handled := m.handleAdversaryStreamMsg(msg); handled {
		return updated, cmd, true
	}

	switch msg.(type) {
	case streamRenderMsg:
		m.flushStreamRender()
		return m, nil, true
	}

	if m.streamHandler == nil || !m.streamHandler.IsActive() {
		switch msg.(type) {
		case llmChunkMsg, llmReasoningChunkMsg, llmDoneMsg, llmIncompleteMsg, llmErrorMsg, llmRetryMsg, llmToolStartMsg, llmToolEndMsg, llmUsageMsg, llmAutoCompactionStartedMsg, llmAutoCompactionAppliedMsg, llmAutoCompactionCancelledMsg, llmAutoCompactionFailedMsg:
			return m, nil, true
		}
	}

	switch msg := msg.(type) {
	case llmUsageMsg:
		updated, cmd := m.handleLLMUsage(msg.usage)
		return updated, cmd, true
	case llmChunkMsg:
		updated, cmd := m.handleLLMChunk(string(msg))
		return updated, cmd, true
	case llmReasoningChunkMsg:
		updated, cmd := m.handleLLMReasoningChunk(string(msg))
		return updated, cmd, true
	case llmDoneMsg:
		updated, cmd := m.handleLLMDone()
		return updated, cmd, true
	case llmIncompleteMsg:
		updated, cmd := m.handleLLMIncomplete(msg.err)
		return updated, cmd, true
	case llmErrorMsg:
		updated, cmd := m.handleLLMError(msg.err)
		return updated, cmd, true
	case llmRetryMsg:
		updated, cmd := m.handleLLMRetry(msg.err, msg.attempt)
		return updated, cmd, true
	case llmToolStartMsg:
		updated, cmd := m.handleToolStart(msg.toolCall)
		return updated, cmd, true
	case llmToolEndMsg:
		updated, cmd := m.handleToolEnd(msg.toolCall)
		return updated, cmd, true
	case llmAutoCompactionStartedMsg, llmAutoCompactionCancelledMsg, llmAutoCompactionFailedMsg:
		return m, m.waitForAsyncEvent(), true
	case llmAutoCompactionAppliedMsg:
		updated, cmd := m.handleAutoCompactionApplied(msg.event)
		return updated, cmd, true
	default:
		return m, nil, false
	}
}

func (m *replModel) handleUpdateCheckMsg(msg updateCheckMsg) {
	if msg.latest == "" {
		return
	}
	m.output.AddEmptyLine()
	m.output.AddStyledLine("  Update available: v"+msg.latest, repltheme.UpdateAvailableStyle)
	m.output.AddEmptyLine()
	updateCmd := "  npm update -g keen-agent\n  or\n  curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh | bash"
	m.output.AddStyledLine(updateCmd, repltheme.UpdateCommandStyle)
	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
}

func (m replModel) handleBtwStreamMsg(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if m.btwStreamHandler == nil || !m.btwStreamHandler.IsActive() {
		switch msg.(type) {
		case btwChunkMsg, btwDoneMsg, btwErrorMsg:
			return m, nil, true
		}
		return m, nil, false
	}

	switch msg := msg.(type) {
	case btwChunkMsg:
		m.btwStreamHandler.HandleChunk(string(msg))
		return m, tea.Batch(m.afterStreamUpdate(), waitForBtwEvent(m.btwStreamHandler.eventCh)), true
	case btwDoneMsg:
		m.flushStreamRender()
		responseLines, _ := m.btwStreamHandler.HandleDone()
		m.btwShowSpinner = false
		m.btwLines = responseLines
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, nil, true
	case btwErrorMsg:
		m.flushStreamRender()
		pendingLines, errMsg := m.btwStreamHandler.HandleError(msg.err)
		m.btwShowSpinner = false
		lines := pendingLines
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			lines = append(lines, "  "+repltheme.ErrorStyle.Render(errMsg))
		}
		m.btwLines = lines
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m replModel) handleAdversaryStreamMsg(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if m.adversary.streamHandler == nil || !m.adversary.streamHandler.IsActive() {
		switch msg.(type) {
		case adversaryChunkMsg, adversaryDoneMsg, adversaryErrorMsg, adversaryToolStartMsg, adversaryToolEndMsg:
			return m, nil, true
		}
		return m, nil, false
	}

	switch msg := msg.(type) {
	case adversaryChunkMsg:
		m.adversary.streamHandler.HandleChunk(string(msg))
		return m, tea.Batch(m.afterStreamUpdate(), waitForAdversaryEvent(m.adversary.streamHandler.eventCh)), true
	case adversaryToolStartMsg:
		m.flushStreamRender()
		m.adversary.streamHandler.HandleToolStart(msg.toolCall)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, waitForAdversaryEvent(m.adversary.streamHandler.eventCh), true
	case adversaryToolEndMsg:
		m.flushStreamRender()
		m.adversary.streamHandler.HandleToolEnd(msg.toolCall)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, waitForAdversaryEvent(m.adversary.streamHandler.eventCh), true
	case adversaryDoneMsg:
		m.flushStreamRender()
		responseLines, _ := m.adversary.streamHandler.HandleDone()
		m.adversary.showSpinner = false
		m.adversary.lines = responseLines
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, nil, true
	case adversaryErrorMsg:
		m.flushStreamRender()
		pendingLines, errMsg := m.adversary.streamHandler.HandleError(msg.err)
		m.adversary.showSpinner = false
		lines := pendingLines
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			lines = append(lines, "  "+repltheme.ErrorStyle.Render(errMsg))
		}
		m.adversary.lines = lines
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, nil, true
	default:
		return m, nil, false
	}
}
