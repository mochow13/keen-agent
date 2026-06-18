package repl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	replcommands "github.com/mochow13/keen-agent/internal/cli/repl/commands"
	replfilesearch "github.com/mochow13/keen-agent/internal/cli/repl/filesearch"
	replhistory "github.com/mochow13/keen-agent/internal/cli/repl/history"
	replmarkdown "github.com/mochow13/keen-agent/internal/cli/repl/markdown"
	reploutput "github.com/mochow13/keen-agent/internal/cli/repl/output"
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	repltheme "github.com/mochow13/keen-agent/internal/cli/repl/theme"
	repltooling "github.com/mochow13/keen-agent/internal/cli/repl/tooling"
	replwidgets "github.com/mochow13/keen-agent/internal/cli/repl/widgets"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/filesystem"
	"github.com/mochow13/keen-agent/internal/llm"
	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
	"github.com/mochow13/keen-agent/internal/providers"
	"github.com/mochow13/keen-agent/internal/session"
	"github.com/mochow13/keen-agent/internal/skills"
)

const (
	defaultWidth            = 120
	inputMinHeight          = 1
	inputMaxHeight          = 15
	copyNotificationTimeout = 2 * time.Second
	copyNotificationMessage = "Copied to clipboard"
)

type replContext struct {
	version    string
	workingDir string
	cfg        *config.ResolvedConfig
	globalCfg  *config.GlobalConfig
	loader     *config.Loader
	registry   *providers.Registry
	mcp        keenmcp.Runtime
}

type replModel struct {
	textarea                  textarea.Model
	viewport                  viewport.Model
	ctx                       *replContext
	mode                      llm.AgentMode
	appState                  *replappstate.AppState
	output                    *reploutput.OutputBuilder
	modelSelection            *replwidgets.Model
	permissionRequester       *replpermissions.Requester
	projectPerms              *config.ProjectPermissions
	diffEmitter               *repltooling.DiffEmitter
	sessions                  *replSessionState
	sessionPicker             *replwidgets.SessionPicker
	suggestion                replwidgets.SuggestionModel
	fileSearcher              *replfilesearch.FileSearcher
	quitting                  bool
	streamHandler             *StreamHandler
	mdRenderer                *replmarkdown.Renderer
	width                     int
	height                    int
	spinner                   spinner.Model
	showSpinner               bool
	loadingText               string
	loadingStartedAt          time.Time
	userScrolled              bool
	streamCancel              context.CancelFunc
	turnMemory                *turnMemoryAccumulator
	isCompacting              bool
	compactionCancel          context.CancelFunc
	contextStatus             contextStatus
	showThinking              bool
	history                   replhistory.InputHistory
	selection                 viewportSelection
	inputSelection            inputSelection
	btwLines                  []string
	btwQuestion               string
	btwStreamHandler          *StreamHandler
	btwStreamCancel           context.CancelFunc
	btwShowSpinner            bool
	btwSpinner                spinner.Model
	bang                      bangState
	adversary                 adversaryState
	lastSession               *session.Summary
	projectPermsErr           error
	initialScreenDone         bool
	copyNotification          string
	copyNotificationExpiresAt time.Time
}

type bangState struct {
	active bool
	events <-chan tea.Msg
	cancel context.CancelFunc
}

type adversaryState struct {
	streamHandler  *StreamHandler
	streamCancel   context.CancelFunc
	lines          []string
	focus          string
	showSpinner    bool
	spinner        spinner.Model
	modelSelection *replwidgets.Model
}

func initialModel(ctx *replContext, llmClient llm.LLMClient, needsSetup bool) replModel {
	ta := textarea.New()
	ta.Placeholder = "What are we building?"
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(defaultWidth - 3)
	ta.DynamicHeight = true
	ta.MinHeight = inputMinHeight
	ta.MaxHeight = inputMaxHeight
	ta.SetHeight(inputMinHeight)
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(3, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return " ▶ "
		}
		return "   "
	})

	styles := ta.Styles()
	styles.Focused.Prompt = repltheme.PromptStyle
	styles.Focused.Text = lipgloss.NewStyle()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Blurred.Prompt = repltheme.InputRuleBlurredStyle
	styles.Blurred.Text = lipgloss.NewStyle()
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	ta.KeyMap.InsertNewline.SetKeys("shift+enter")
	ta.KeyMap.InsertNewline.SetEnabled(true)

	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(repltheme.PrimaryColor)

	bs := spinner.New()
	bs.Spinner = spinner.MiniDot
	bs.Style = lipgloss.NewStyle().Foreground(repltheme.AccentColor)

	as := spinner.New()
	as.Spinner = spinner.Points
	as.Style = lipgloss.NewStyle().Foreground(repltheme.SecondaryColor)

	appState := replappstate.New(llmClient, ctx.workingDir)

	projectPerms, projectPermsErr := config.LoadProjectPermissions(ctx.workingDir)
	if projectPermsErr != nil {
		projectPerms = config.NewProjectPermissions()
	}

	permissionRequester := replpermissions.NewRequester(projectPerms)
	diffEmitter := repltooling.NewDiffEmitter()
	sessions := newReplSessionState(ctx.workingDir)

	fileGitAwareness := filesystem.NewGitAwareness()
	_ = fileGitAwareness.LoadGitignore(filepath.Join(ctx.workingDir, ".gitignore"))
	fileGuard := filesystem.NewGuard(ctx.workingDir, fileGitAwareness)
	fileSearcher := replfilesearch.NewFileSearcher(ctx.workingDir, fileGuard)

	repltooling.SetupToolRegistry(ctx.workingDir, appState, permissionRequester, diffEmitter, ctx.mcp, ctx.cfg)

	mdRenderer, err := replmarkdown.New(defaultWidth)

	if err != nil {
		mdRenderer = nil
	}

	output := reploutput.NewOutputBuilder(defaultWidth, ctx.workingDir)
	var lastSession *session.Summary
	if summaries, err := sessions.listSessions(); err == nil && len(summaries) > 0 {
		lastSession = &summaries[0]
	}

	vp := viewport.New(viewport.WithWidth(defaultWidth), viewport.WithHeight(24))

	model := replModel{
		textarea:            ta,
		viewport:            vp,
		ctx:                 ctx,
		mode:                llm.ModeBuild,
		appState:            appState,
		output:              output,
		spinner:             s,
		btwSpinner:          bs,
		streamHandler:       NewStreamHandler(mdRenderer),
		mdRenderer:          mdRenderer,
		permissionRequester: permissionRequester,
		projectPerms:        projectPerms,
		diffEmitter:         diffEmitter,
		sessions:            sessions,
		suggestion:          replwidgets.NewSuggestionModel(),
		fileSearcher:        fileSearcher,
		showThinking:        true,
		btwStreamHandler:    NewStreamHandler(mdRenderer),
		adversary: adversaryState{
			spinner:       as,
			streamHandler: NewStreamHandler(mdRenderer),
		},
		lastSession:     lastSession,
		projectPermsErr: projectPermsErr,
	}
	if ctx.globalCfg != nil && ctx.globalCfg.ShowThinking != nil {
		model.showThinking = *ctx.globalCfg.ShowThinking
	}
	model.streamHandler.workingDir = ctx.workingDir
	model.streamHandler.showThinking = model.showThinking

	historyDir, err := os.UserHomeDir()
	if err == nil {
		historyDir = filepath.Join(historyDir, ".keen-agent")
		if mkdirErr := os.MkdirAll(historyDir, 0755); mkdirErr == nil {
			_ = model.history.LoadFromFile(filepath.Join(historyDir, "input-history"))
		}
	}

	model.refreshContextStatus()

	if needsSetup {
		welcomeStyle := lipgloss.NewStyle().Foreground(repltheme.PrimaryColor).Bold(true)
		model.output.AddEmptyLine()
		model.output.AddStyledLine(welcomeStyle.Render("👋 Welcome to Keen Agent!"), lipgloss.NewStyle())
		model.output.AddEmptyLine()
		model.output.AddEmptyLine()
		model = model.startModelSelection()
	} else {
		model.updateViewportContent()
	}

	return model
}

func (m replModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, checkForUpdate(m.ctx.version), waitForMCPStartup(m.ctx.mcp))
}

func (m *replModel) handleEnterKey() (replModel, tea.Cmd) {
	input := m.textarea.Value()
	if input == "" {
		return *m, nil
	}

	if input == replcommands.Adversary || strings.HasPrefix(input, replcommands.Adversary+" ") {
		if m.showSpinner {
			return *m, nil
		}
		m.history.Push(input)
		m.textarea.Reset()
		result, cmd := m.handleAdversaryCommand(input)
		return result, cmd
	}

	if input == replcommands.Btw || strings.HasPrefix(input, replcommands.Btw+" ") {
		m.history.Push(input)
		m.textarea.Reset()
		result, cmd := m.handleBtwCommand(input)
		return result, cmd
	}

	if strings.HasPrefix(input, "!") {
		if m.streamHandler.IsActive() || m.bang.active {
			return *m, nil
		}
		m.history.Push(input)
		m.textarea.Reset()
		result, cmd := m.handleBangCommand(input)
		return result, cmd
	}

	if m.streamHandler.IsActive() {
		return *m, nil
	}

	if m.isCompacting {
		return *m, nil
	}

	m.flushBtwToOutput()
	m.flushAdversaryToOutput()
	m.output.AddUserInput(input, repltheme.PromptStyle)
	m.history.Push(input)

	if updated, cmd, handled := m.dispatchCommand(input); handled {
		return updated, cmd
	}

	if activated, ok := m.activateSkillInput(input); ok {
		input = activated
	}

	if !m.appState.IsClientReady(m.ctx.cfg) {
		m.output.AddError("LLM client not initialized. Use /model to configure.", repltheme.ErrorStyle)
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	if err := m.sessions.appendUserMessage(input); err != nil {
		m.output.AddError("Session persistence failed: "+err.Error(), repltheme.ErrorStyle)
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.appState.AddMessage(llm.RoleUser, input)

	ctx := m.startStreamContext()
	eventCh, err := m.appState.StreamChat(ctx, m.ctx.cfg, llm.StreamOptions{SessionID: m.sessions.currentID()})
	if err != nil {
		m.clearStreamCancel()
		m.output.AddError(err.Error(), repltheme.ErrorStyle)
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.startLoading(nextLoadingText())
	m.startAssistantTurnMemory()
	m.streamHandler.Start(eventCh, m.loadingText)
	m.textarea.Reset()
	m.userScrolled = false
	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.viewport.GotoBottom()

	return *m, tea.Batch(m.spinner.Tick, m.waitForAsyncEvent())
}

func (m *replModel) activateSkillInput(input string) (string, bool) {
	if !strings.HasPrefix(input, "/") || strings.Contains(input, "\n") {
		return "", false
	}
	fields := strings.Fields(strings.TrimPrefix(input, "/"))
	if len(fields) == 0 {
		return "", false
	}

	skill, ok := m.appState.FindEnabledSkill(fields[0])
	if !ok {
		return "", false
	}
	msg, err := skills.ActivationMessage(skill, fields[1:])
	if err != nil {
		return "", false
	}
	return msg, true
}

func (m *replModel) updateViewportContent() {
	if m.viewport.Width() == 0 {
		return
	}

	contentWidth := m.width
	if contentWidth <= 0 {
		contentWidth = m.viewport.Width()
	}

	var content strings.Builder

	if m.output != nil && !m.output.IsEmpty() {
		content.WriteString(m.output.Join())
	}

	if m.streamHandler != nil && m.streamHandler.IsActive() {
		content.WriteString(m.streamHandler.View(contentWidth))
	}

	if m.btwStreamHandler != nil && m.btwStreamHandler.IsActive() {
		content.WriteString(m.renderBtwInline(contentWidth))
	} else if m.btwLines != nil {
		content.WriteString(m.renderBtwInlineFinished(contentWidth))
	}

	if m.adversary.streamHandler != nil && m.adversary.streamHandler.IsActive() {
		content.WriteString(m.renderAdversaryInline(contentWidth))
	} else if m.adversary.lines != nil {
		content.WriteString(m.renderAdversaryInlineFinished(contentWidth))
	}

	if m.modelSelection != nil {
		content.WriteString(formatModelSelectionCard(m.modelSelection, m.viewport.Width()))
	}

	if m.adversary.modelSelection != nil {
		content.WriteString(formatModelSelectionCard(m.adversary.modelSelection, m.viewport.Width()))
	}

	if m.sessionPicker != nil {
		content.WriteString(replwidgets.FormatSessionPickerCard(m.sessionPicker, m.viewport.Width(), m.viewport.Height()))
	}

	viewportContent := content.String()
	m.viewport.SetContent(viewportContent)
	m.selection.setContent(viewportContent)
}

func (m replModel) waitForAsyncEvent() tea.Cmd {
	if m.streamHandler == nil || !m.streamHandler.IsActive() || m.streamHandler.eventCh == nil {
		return nil
	}
	var permissionCh <-chan *replpermissions.Request
	if m.permissionRequester != nil {
		permissionCh = m.permissionRequester.GetRequestChan()
	}
	var diffCh <-chan repltooling.DiffRequest
	if m.diffEmitter != nil {
		diffCh = m.diffEmitter.GetDiffChan()
	}
	return waitForAsyncEvent(
		m.streamHandler.eventCh,
		permissionCh,
		diffCh,
	)
}

func (m replModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedModel, cmd := m.updateNormalMode(msg)
	return &updatedModel, cmd
}

func (m replModel) updateNormalMode(msg tea.Msg) (replModel, tea.Cmd) {
	if updated, cmd, handled := m.handleBangMsg(msg); handled {
		return updated, cmd
	}

	if updated, cmd, handled := m.handleLLMStreamMsg(msg); handled {
		return updated, cmd
	}

	if updated, cmd, handled := m.consumeModelSelectionResult(msg); handled {
		return updated, cmd
	}

	if updated, cmd, handled := m.consumeAdversaryModelSelectionResult(msg); handled {
		return updated, cmd
	}

	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.applyWindowSize(sizeMsg)
		if m.modelSelection != nil {
			m.updateViewportContent()
		}
		return m, nil
	}

	if m.modelSelection != nil {
		return m.handleKeyMsg(msg)
	}

	if m.adversary.modelSelection != nil {
		return m.handleKeyMsg(msg)
	}

	switch msg := msg.(type) {
	case compactionDoneMsg:
		return m.handleCompactionDone()
	case compactionErrMsg:
		return m.handleCompactionError(msg.err)
	case updateCheckMsg:
		m.handleUpdateCheckMsg(msg)
		return m, nil
	case mcpStartupStatusMsg:
		m.handleMCPStartupStatus(msg.Statuses)
		return m, nil
	case mcpConnectDoneMsg:
		m.handleMCPConnectDone(msg)
		return m, nil
	case copyNotificationExpiredMsg:
		if m.copyNotificationExpiresAt.UnixNano() == msg.expiresAt {
			m.copyNotification = ""
			m.copyNotificationExpiresAt = time.Time{}
		}
		return m, nil
	case diffReadyMsg:
		m.streamHandler.HandleDiff(msg.req.Lines)
		close(msg.req.Done)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, m.waitForAsyncEvent()

	case permissionReadyMsg:
		m.streamHandler.HandlePermissionRequest(msg.req)
		m.updateViewportContent()
		m.scrollToBottomIfFollowing()
		return m, m.waitForAsyncEvent()

	case spinner.TickMsg:
		if updated, cmd, handled := m.handleSpinnerTick(msg); handled {
			return updated, cmd
		}

	case tea.WindowSizeMsg:
		m.applyWindowSize(msg)
		return m, nil

	case tea.KeyPressMsg:
		if m.inputSelection.hasSelection() {
			if msg.String() == keyEsc {
				m.inputSelection.clear()
				return m, nil
			}
			m.inputSelection.clear()
		}
		if m.selection.hasSelection() {
			if msg.String() == keyEsc {
				m.selection.clear()
				return m, nil
			}
		}
		return m.handleKeyMsg(msg)

	case tea.MouseClickMsg:
		if handled, cmd := m.handleMouseDown(msg); handled {
			return m, cmd
		}

	case tea.MouseMotionMsg:
		if handled := m.handleMouseDrag(msg); handled {
			return m, nil
		}

	case tea.MouseReleaseMsg:
		if handled, cmd := m.handleMouseUp(); handled {
			return m, cmd
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.viewport.ScrollUp(3)
			m.userScrolled = !m.viewport.AtBottom()
		case tea.MouseWheelDown:
			m.viewport.ScrollDown(3)
			m.userScrolled = !m.viewport.AtBottom()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.adjustTextareaHeight()
	return m, cmd
}

func (m replModel) consumeModelSelectionResult(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if m.modelSelection == nil {
		return m, nil, false
	}

	if replwidgets.IsComplete(msg) {
		modelName := displayModelName(m.modelSelection.SelectedProvider, m.modelSelection.SelectedModel)
		successMsg := "✓ Updated to " + m.modelSelection.SelectedProvider + " / " + modelName
		m.output.AddStyledLine("  "+successMsg, repltheme.HighlightStyle)
		m.output.AddEmptyLine()
		m.modelSelection = nil
		m.refreshContextStatus()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil, true
	}

	if replwidgets.IsCancel(msg) {
		cancelStyle := lipgloss.NewStyle().Foreground(repltheme.MutedColor)
		m.output.AddStyledLine("  Model selection cancelled", cancelStyle)
		m.output.AddEmptyLine()
		m.modelSelection = nil
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil, true
	}

	return m, nil, false
}

func (m replModel) consumeAdversaryModelSelectionResult(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if m.adversary.modelSelection == nil {
		return m, nil, false
	}

	if replwidgets.IsComplete(msg) {
		modelName := displayModelName(m.adversary.modelSelection.SelectedProvider, m.adversary.modelSelection.SelectedModel)
		successMsg := "✓ Adversary set to " + m.adversary.modelSelection.SelectedProvider + " / " + modelName
		m.output.AddStyledLine("  "+successMsg, repltheme.HighlightStyle)
		m.output.AddEmptyLine()
		m.adversary.modelSelection = nil
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil, true
	}

	if replwidgets.IsCancel(msg) {
		cancelStyle := lipgloss.NewStyle().Foreground(repltheme.MutedColor)
		m.output.AddStyledLine("  Adversary model selection cancelled", cancelStyle)
		m.output.AddEmptyLine()
		m.adversary.modelSelection = nil
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil, true
	}

	return m, nil, false
}

func (m replModel) handleSpinnerTick(msg spinner.TickMsg) (replModel, tea.Cmd, bool) {
	if !m.showSpinner && !m.btwShowSpinner && !m.adversary.showSpinner {
		return m, nil, false
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	if m.showSpinner {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.btwShowSpinner {
		m.btwSpinner, cmd = m.btwSpinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.adversary.showSpinner {
		m.adversary.spinner, cmd = m.adversary.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}
	m.updateViewportContent()
	return m, tea.Batch(cmds...), true
}

func (m replModel) View() tea.View {
	var content string

	if m.quitting {
		content = lipgloss.NewStyle().Foreground(repltheme.MutedColor).Render("\n  Goodbye!\n")
	} else {
		var view strings.Builder

		viewportView := m.selection.render(m.viewport.View(), m.viewport.Width(), m.viewport.Height(), m.viewport.YOffset())
		view.WriteString(viewportView)
		view.WriteString("\n")

		if m.showSpinner {
			elapsed := time.Duration(0)
			if !m.loadingStartedAt.IsZero() {
				elapsed = time.Since(m.loadingStartedAt)
			}
			spinnerText := " " + m.spinner.View() + " " + renderLoadingText(m.loadingText, elapsed)
			view.WriteString("\n")
			view.WriteString(spinnerText)
			view.WriteString("\n")
		}

		shellMode := strings.HasPrefix(m.textarea.Value(), "!")
		styles := m.textarea.Styles()
		if shellMode {
			styles.Focused.Prompt = repltheme.ShellPromptStyle
		} else if m.currentMode() == llm.ModePlan {
			styles.Focused.Prompt = repltheme.PromptPlanStyle
		} else {
			styles.Focused.Prompt = repltheme.PromptStyle
		}
		m.textarea.SetStyles(styles)

		textareaView := m.inputSelection.render(m.textarea.View(), m.textarea.Width()+inputPromptWidth, m.textarea.Height(), m.textarea.ScrollYOffset(), inputPromptWidth)
		view.WriteString(renderInputArea(textareaView, m.width, m.textarea.Focused(), shellMode, m.currentMode()))
		view.WriteString("\n")
		if m.suggestion.Visible() {
			view.WriteString(m.suggestion.View(m.width))
			view.WriteString("\n")
		}
		view.WriteString(m.inputMetaView())

		content = view.String()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m replModel) inputMetaView() string {
	model := "-"

	if m.ctx != nil && m.ctx.cfg != nil {
		if m.ctx.cfg.Model != "" {
			model = displayModelName(m.ctx.cfg.Provider, m.ctx.cfg.Model)
		}
	}

	modelText := repltheme.HighlightStyle.Render(model)
	directory := repltheme.HighlightStyle.Render(" " + abbreviateHome(m.ctx.workingDir))

	thinkingText := ""
	if m.ctx != nil && m.ctx.cfg != nil && m.ctx.cfg.ThinkingEffort != "" && m.ctx.registry != nil {
		if modelMeta, ok := m.ctx.registry.GetModel(m.ctx.cfg.Provider, m.ctx.cfg.Model); ok && modelMeta.SupportsThinkingEffort() {
			effortValue := m.ctx.cfg.ThinkingEffort
			if m.ctx.cfg.Provider == config.ProviderAnthropic {
				effortValue += " (adaptive)"
			}
			thinkingText = repltheme.MetaLabelStyle.Render("thinking:") + " " + repltheme.HighlightStyle.Render(effortValue)
		}
	}

	contextText := renderContextStatus(m.contextStatus)

	copyText := ""
	if m.copyNotification != "" {
		copyText = repltheme.AccentStyle.Render(m.copyNotification)
	}

	timerText := ""
	if m.showSpinner {
		timerText = repltheme.LoadingTimerStyle.Render("⏱ " + m.loadingElapsedText())
	}

	parts := []string{directory, modelText}
	if thinkingText != "" {
		parts = append(parts, thinkingText)
	}
	parts = append(parts, contextText)
	if copyText != "" {
		parts = append(parts, copyText)
	}
	if timerText != "" {
		parts = append(parts, timerText)
	}
	left := strings.Join(parts, " • ")
	right := ""
	if m.contextStatus.ShouldSuggestCompaction() {
		right = repltheme.CompactionSuggestionStyle.Render("Try /compact")
	}

	const leftPad = "  "
	if m.width <= 0 {
		if right == "" {
			return leftPad + left
		}
		return leftPad + left + "   " + right
	}

	available := m.width - lipgloss.Width(leftPad)
	if right == "" {
		return leftPad + left
	}
	if available < lipgloss.Width(left)+lipgloss.Width(right)+1 {
		right = ""
	}
	if right == "" {
		return leftPad + left
	}
	if available <= lipgloss.Width(right)+1 {
		return leftPad + left
	}

	space := available - lipgloss.Width(left) - lipgloss.Width(right)
	if space >= 1 {
		return leftPad + left + strings.Repeat(" ", space) + right
	}

	return leftPad + left
}

func (m *replModel) replayLoadedSession(loaded *session.LoadedSession) {
	if loaded == nil {
		return
	}

	replay := newSessionReplay(m.width, m.mdRenderer, m.ctx.workingDir)
	replay.handler.showThinking = m.showThinking

	for _, event := range loaded.Events {
		replay.applyEvent(event)
	}

	replay.flushDone()

	m.output = replay.output
	m.appState.ReplaceMessages(session.BuildConversation(loaded.Events))
	m.history.Reset()
	m.sessionPicker = nil
	m.contextStatus.ResetTotals()
	m.refreshContextStatus()
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func RunREPL(
	version string,
	workingDir string,
	cfg *config.ResolvedConfig,
	loader *config.Loader,
	globalCfg *config.GlobalConfig,
	registry *providers.Registry,
	needsSetup bool,
	mcpRuntime keenmcp.Runtime,
) error {
	ctx := &replContext{
		version:    version,
		workingDir: workingDir,
		cfg:        cfg,
		globalCfg:  globalCfg,
		loader:     loader,
		registry:   registry,
		mcp:        mcpRuntime,
	}

	var llmClient llm.LLMClient
	if cfg.Model != "" && (!config.RequiresAPIKey(cfg.Provider) || cfg.APIKey != "") {
		client, err := llm.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize LLM client: %w", err)
		}
		llmClient = client
	}

	m := initialModel(ctx, llmClient, needsSetup)
	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
