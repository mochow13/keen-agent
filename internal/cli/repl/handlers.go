package repl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
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
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleLLMReasoningChunk(chunk string) (replModel, tea.Cmd) {
	m.streamHandler.HandleReasoningChunk(chunk)
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleLLMDone() (replModel, tea.Cmd) {
	if m.isCompacting {
		return m.handleCompactionDone()
	}
	segments := cloneStreamSegments(m.streamHandler.segments)
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
	return *m, nil
}

func (m *replModel) handleLLMIncomplete(err error) (replModel, tea.Cmd) {
	segments := cloneStreamSegments(m.streamHandler.segments)
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
	return *m, nil
}

func (m *replModel) handleLLMError(err error) (replModel, tea.Cmd) {
	if m.isCompacting {
		return m.handleCompactionError(err)
	}
	segments := cloneStreamSegments(m.streamHandler.segments)
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
	m.streamHandler.RewindForRetry()
	m.loadingText = fmt.Sprintf("Retrying (attempt %d)...", attempt)
	m.streamHandler.SetLoadingText(m.loadingText)
	m.updateViewportContent()
	m.scrollToBottomIfFollowing()
	return *m, m.waitForAsyncEvent()
}

func (m *replModel) handleCompactionDone() (replModel, tea.Cmd) {
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
	return *m, nil
}

func (m *replModel) handleCompactionError(err error) (replModel, tea.Cmd) {
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
	m.recordToolMemory(toolCall)
	if toolCall.Name == "bash" {
		m.streamHandler.HandleBashEnd(toolCall)
	} else {
		m.streamHandler.HandleToolEnd(toolCall)
		m.loadingText = nextLoadingText()
		m.streamHandler.SetLoadingText(m.loadingText)
	}
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
		if cur := m.suggestion.Current(); cur != nil {
			m.textarea.SetValue(cur.Name)
		} else if first := m.suggestion.First(); first != nil {
			m.textarea.SetValue(first.Name)
		}
		if keyMsg.String() == keyEnter {
			m.suggestion.Refresh("")
		} else {
			m.suggestion.Refresh(m.textarea.Value())
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
			return true, *m, nil
		}
	}
	return false, *m, nil
}

func (m *replModel) handleKeyMsg(msg tea.Msg) (replModel, tea.Cmd) {
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
		case "up", "k", "down", "j", keyEnter, keyEsc:
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
			if strings.HasPrefix(input, "/") {
				m.suggestion.RefreshWithSkills(input, m.skillSuggestions())
			} else {
				m.refreshFileSuggestions(input)
			}
			m.adjustTextareaHeight()
			return *m, tea.Batch(cmd, textCmd)
		}
	}

	switch keyMsg.String() {
	case keyEnter:
		return m.handleEnterKey()
	case keyCtrlC, keyCtrlD:
		if m.bang.active {
			m.cancelBangCommand()
			return *m, nil
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
	if strings.HasPrefix(input, "/") {
		m.suggestion.RefreshWithSkills(input, m.skillSuggestions())
	} else {
		m.refreshFileSuggestions(input)
	}
	m.adjustTextareaHeight()
	return *m, cmd
}

func (m *replModel) skillSuggestions() []replwidgets.SuggestionItem {
	skillList := m.appState.SkillSuggestions()
	items := make([]replwidgets.SuggestionItem, 0, len(skillList))
	for _, skill := range skillList {
		items = append(items, replwidgets.SuggestionItem{Name: "/" + skill.Name, Description: skill.Description})
	}
	return items
}

func (m *replModel) refreshFileSuggestions(input string) {
	if m.fileSearcher == nil {
		m.suggestion.Hide()
		return
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
	} else {
		m.suggestion.Hide()
	}
}

func (m *replModel) interruptStream(message string) {
	if m.streamCancel != nil {
		m.streamCancel()
		m.clearStreamCancel()
	}

	m.stopLoading()

	segments := cloneStreamSegments(m.streamHandler.segments)
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
	case keyUp, "k", keyShiftUp:
		m.sessionPicker.Move(-1)
		m.updateViewportContent()
	case keyDown, "j", keyShiftDown:
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
	case "up", "k":
		m.streamHandler.MovePendingCursor(-1)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
	case "down", "j":
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

	if m.streamHandler == nil || !m.streamHandler.IsActive() {
		switch msg.(type) {
		case llmChunkMsg, llmReasoningChunkMsg, llmDoneMsg, llmIncompleteMsg, llmErrorMsg, llmRetryMsg, llmToolStartMsg, llmToolEndMsg, llmUsageMsg:
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
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, waitForBtwEvent(m.btwStreamHandler.eventCh), true
	case btwDoneMsg:
		responseLines, _ := m.btwStreamHandler.HandleDone()
		m.btwShowSpinner = false
		m.btwLines = responseLines
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, nil, true
	case btwErrorMsg:
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
		case adversaryChunkMsg, adversaryDoneMsg, adversaryErrorMsg:
			return m, nil, true
		}
		return m, nil, false
	}

	switch msg := msg.(type) {
	case adversaryChunkMsg:
		m.adversary.streamHandler.HandleChunk(string(msg))
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, waitForAdversaryEvent(m.adversary.streamHandler.eventCh), true
	case adversaryDoneMsg:
		responseLines, _ := m.adversary.streamHandler.HandleDone()
		m.adversary.showSpinner = false
		m.adversary.lines = responseLines
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, nil, true
	case adversaryErrorMsg:
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
