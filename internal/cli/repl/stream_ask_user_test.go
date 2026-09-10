package repl

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	replaskuser "github.com/mochow13/keen-agent/internal/cli/repl/askuser"
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	repltheme "github.com/mochow13/keen-agent/internal/cli/repl/theme"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/tools"
)

func testAskUserState() askUserState {
	return askUserState{
		input: newAskUserInput(),
		request: &replaskuser.Request{Questionnaire: tools.AskUserRequest{Questions: []tools.AskUserQuestion{
			{Question: "Choose one", Options: []string{"Recommended", "Alternative"}},
			{Question: "Choose two", Options: []string{"First", "Second"}},
		}}},
	}
}

func TestRenderAskUserCardIncludesGuidanceAndRecommendation(t *testing.T) {
	rendered := renderAskUserCard(testAskUserState(), 80)
	plain := ansi.Strip(rendered)
	for _, text := range []string{"Question 1 of 2", "› • Recommended", "(recommended)", "  • Type your answer", "↑/↓ navigate · Enter select · Esc cancel"} {
		if !strings.Contains(plain, text) {
			t.Fatalf("card missing %q: %q", text, plain)
		}
	}
	if !strings.Contains(rendered, repltheme.AskUserProgressStyle.Render("Question 1 of 2")) {
		t.Fatal("question progress should use the primary style")
	}
	if !strings.Contains(rendered, repltheme.AskUserSelectedStyle.Render("› ")) {
		t.Fatal("selection cursor should use the secondary style")
	}
	if !strings.Contains(rendered, repltheme.AskUserBadgeStyle.Render("(recommended)")) {
		t.Fatal("recommendation badge should use a faint grey style")
	}
	lines := strings.Split(plain, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Question 1 of 2") || strings.Contains(line, "Choose one") || strings.Contains(line, "• Alternative") {
			if !strings.HasPrefix(line, strings.Repeat(" ", askUserHorizontalMargin)) {
				t.Fatalf("questionnaire content should align with rule: %q", line)
			}
		}
	}
	if !strings.Contains(plain, "• Alternative") || !strings.Contains(plain, "• Type your answer") {
		t.Fatalf("options should have bullets: %q", plain)
	}
	if !strings.Contains(rendered, repltheme.AskUserRuleStyle.Render(strings.Repeat("─", 80))) {
		t.Fatal("questionnaire rule should use the ask-user rule style")
	}
}

func TestAskUserCustomOptionStartsEditingWhenSelected(t *testing.T) {
	m := newTestModel()
	m.askUser = testAskUserState()
	m.askUser.requester = replaskuser.NewRequester()

	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	if !m.askUser.editing {
		t.Fatal("selecting the custom option should start editing")
	}
	rendered := renderAskUserCard(m.askUser, 80)
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "› • Type your answer") {
		t.Fatalf("selected custom option should show a text cursor: %q", plain)
	}

	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.askUser.input.Value() != "x" || !m.askUser.editing {
		t.Fatalf("selected custom option should accept typing: %#v", m.askUser)
	}
}

func TestAskUserSelectionKeepsOptionBulletColumnFixed(t *testing.T) {
	state := testAskUserState()
	unselected := ansi.Strip(renderAskUserCard(state, 80))
	state.selected = 1
	selected := ansi.Strip(renderAskUserCard(state, 80))

	if !strings.Contains(unselected, "› • Recommended") || !strings.Contains(unselected, "  • Alternative") {
		t.Fatalf("unexpected initial cursor layout: %q", unselected)
	}
	if !strings.Contains(selected, "  • Recommended") || !strings.Contains(selected, "› • Alternative") {
		t.Fatalf("cursor should move independently from bullets: %q", selected)
	}
}

func TestAskUserTypingReplacesCustomLabel(t *testing.T) {
	m := newTestModel()
	m.askUser = testAskUserState()
	m.askUser.requester = replaskuser.NewRequester()

	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: 'i', Text: "i"})
	if m.askUser.input.Value() != "hi" || !m.askUser.editing {
		t.Fatalf("draft = %q, editing = %t", m.askUser.input.Value(), m.askUser.editing)
	}
	if got := ansi.Strip(renderAskUserCard(m.askUser, 80)); strings.Contains(got, "Type your answer") || !strings.Contains(got, "› • hi") || !strings.Contains(got, "Enter submit · Esc cancel") {
		t.Fatalf("custom answer should replace label: %q", got)
	}
	rendered := renderAskUserCard(m.askUser, 80)
	if !strings.Contains(rendered, styleColorPrefix(repltheme.AskUserTypedStyle)) {
		t.Fatal("typed custom answer should use the secondary color")
	}
}

func TestAskUserUpDownNavigateOptions(t *testing.T) {
	m := newTestModel()
	m.askUser = testAskUserState()
	m.askUser.requester = replaskuser.NewRequester()

	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.askUser.selected != 1 {
		t.Fatalf("selected after down = %d, want 1", m.askUser.selected)
	}
	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.askUser.selected != 2 {
		t.Fatalf("selected after second down = %d, want custom row", m.askUser.selected)
	}
	m, _ = m.handleAskUserKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.askUser.selected != 1 {
		t.Fatalf("selected after up = %d, want 1", m.askUser.selected)
	}
}

func TestAskUserPasteInsertsIntoCustomAnswer(t *testing.T) {
	m := newTestModel()
	m.askUser = testAskUserState()
	m.askUser.requester = replaskuser.NewRequester()

	m, _ = m.updateNormalMode(tea.PasteMsg{Content: "pasted answer"})
	if got := m.askUser.input.Value(); got != "pasted answer" {
		t.Fatalf("paste should fill the custom answer, got %q", got)
	}
	if !m.askUser.editing {
		t.Fatal("paste should switch to editing the custom answer")
	}
	if m.textarea.Value() != "" {
		t.Fatalf("paste should not leak into the main textarea, got %q", m.textarea.Value())
	}
}

func TestAskUserResolvedSummaryKeepsAnswers(t *testing.T) {
	state := testAskUserState()
	state.answers = []string{"Recommended", "Second"}
	state.resolve(nil, false)
	plain := ansi.Strip(renderAskUserCard(state, 80))
	for _, text := range []string{"Answers provided", "• Choose one: Recommended", "• Choose two: Second"} {
		if !strings.Contains(plain, text) {
			t.Fatalf("summary missing %q: %q", text, plain)
		}
	}
	rendered := renderAskUserCard(state, 80)
	if !strings.Contains(rendered, repltheme.AssistantStyle.Render("Choose one: Recommended")) {
		t.Fatal("resolved question and answer should use the assistant text color")
	}
}

func TestAskUserResolvedSummaryIsOrderedInStream(t *testing.T) {
	m := newTestModel()
	m.streamHandler.Start(make(chan llm.StreamEvent), "Loading...")
	m.streamHandler.HandleChunk("Before")
	m.askUser = testAskUserState()
	m.streamHandler.SetAskUser(&m.askUser)
	m.askUser.answers = []string{"Recommended", "Second"}
	m.askUser.resolve(nil, false)
	m.appendResolvedAskUserSegment()
	m.streamHandler.HandleChunk("After")

	plain := ansi.Strip(m.streamHandler.View(80))
	before := strings.Index(plain, "Before")
	answer := strings.Index(plain, "• Choose two: Second")
	after := strings.Index(plain, "After")
	if !(before >= 0 && before < answer && answer < after) {
		t.Fatalf("questionnaire should retain stream order: %q", plain)
	}
}

func TestAskUserResolvedSummaryUsesViewportWidth(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.streamHandler.Start(make(chan llm.StreamEvent), "Loading...")
	m.askUser = testAskUserState()
	m.streamHandler.SetAskUser(&m.askUser)
	m.askUser.answers = []string{"Recommended", "Second"}
	m.askUser.resolve(nil, false)
	m.appendResolvedAskUserSegment()
	expectedRule := strings.Repeat("─", m.width)
	if !strings.Contains(ansi.Strip(m.streamHandler.View(m.width)), expectedRule) {
		t.Fatalf("summary should use the active questionnaire width: %q", ansi.Strip(m.streamHandler.View(m.width)))
	}
	plain := ansi.Strip(m.streamHandler.View(m.width))
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Answers provided") || strings.Contains(line, "Choose one: Recommended") {
			if !strings.HasPrefix(line, strings.Repeat(" ", askUserHorizontalMargin)) {
				t.Fatalf("summary content should retain its horizontal margin: %q", line)
			}
		}
	}
}

func TestAskUserKeyMsgTakesPriorityOverPermissionPrompt(t *testing.T) {
	m := newTestModel()
	m.askUser = testAskUserState()
	m.askUser.requester = replaskuser.NewRequester()
	m.streamHandler.Start(make(chan llm.StreamEvent), "Loading...")
	m.streamHandler.HandlePermissionRequest(&replpermissions.Request{ToolName: "bash"})

	m, _ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.askUser.selected != 1 {
		t.Fatalf("ask-user key should be handled before permission prompt, selected = %d", m.askUser.selected)
	}
}

func styleColorPrefix(style lipgloss.Style) string {
	return style.Render("")
}
