package repl

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestViewportSelection_SelectedTextAcrossLines(t *testing.T) {
	var selection viewportSelection
	selection.setContent("alpha beta\ngamma delta\nthird")
	selection.start(6, 0, 0)
	selection.drag(5, 1, 0)

	if got := selection.selectedText(); got != "beta\ngamma" {
		t.Fatalf("expected selected text, got %q", got)
	}
}

func TestViewportSelection_SelectWord(t *testing.T) {
	var selection viewportSelection
	selection.setContent("hello world")

	if !selection.selectWord(7, 0, 0) {
		t.Fatal("expected word selection")
	}
	if got := selection.selectedText(); got != "world" {
		t.Fatalf("expected selected word, got %q", got)
	}
}

func TestViewportSelection_RenderHighlightsVisibleSelection(t *testing.T) {
	var selection viewportSelection
	selection.setContent("hello world")
	selection.start(0, 0, 0)
	selection.drag(5, 0, 0)

	rendered := selection.render("hello world", 20, 1, 0)
	if rendered == "hello world" {
		t.Fatal("expected rendered selection to add styling")
	}
	if !strings.Contains(rendered, "hello") {
		t.Fatalf("expected highlighted view to preserve content, got %q", rendered)
	}
}

func TestReplSelectionMouseDragCopiesOnRelease(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()
	m.blurInput()

	updated, cmd := m.updateNormalMode(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command on mouse down")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseMotionMsg(tea.Mouse{X: 5, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command while dragging")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseReleaseMsg(tea.Mouse{X: 5, Y: 0, Button: tea.MouseLeft}))
	if cmd == nil {
		t.Fatal("expected copy command on mouse release")
	}
	if updated.copyNotification != copyNotificationMessage {
		t.Fatalf("expected copy notification, got %q", updated.copyNotification)
	}
	if got := updated.selection.selectedText(); got != "hello" {
		t.Fatalf("expected selected text to remain available, got %q", got)
	}
}

func TestReplSelectionMouseClickFocusesViewport(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()

	updated, cmd := m.updateNormalMode(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command on viewport mouse down")
	}
	if updated.textarea.Focused() {
		t.Fatal("expected viewport click to blur input")
	}
}

func TestReplSelectionCtrlCDoesNotCopySelection(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()
	m.selection.start(0, 0, 0)
	m.selection.drag(5, 0, 0)

	updated, cmd := m.updateNormalMode(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected ctrl+c to keep normal quit behavior")
	}
	if !updated.quitting {
		t.Fatal("expected ctrl+c with selection to quit")
	}
}

func TestReplSelectionCmdCDoesNotCopySelection(t *testing.T) {
	m := newTestModel()
	m.output.AddLine("hello world")
	m.updateViewportContent()
	m.selection.start(0, 0, 0)
	m.selection.drag(5, 0, 0)

	updated, cmd := m.updateNormalMode(tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper})
	if cmd != nil {
		t.Fatal("expected no copy command for cmd+c with active selection")
	}
	if updated.quitting {
		t.Fatal("expected cmd+c not to quit")
	}
}

func TestReplInputSelectionMouseDragCopiesOnRelease(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	m.blurInput()
	textY := m.inputAreaTop() + 1

	updated, cmd := m.updateNormalMode(tea.MouseClickMsg(tea.Mouse{X: inputPromptWidth, Y: textY, Button: tea.MouseLeft}))
	if cmd == nil {
		t.Fatal("expected focus command on input mouse down")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseMotionMsg(tea.Mouse{X: inputPromptWidth + 5, Y: textY, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command while dragging input selection")
	}
	updated, cmd = updated.updateNormalMode(tea.MouseReleaseMsg(tea.Mouse{X: inputPromptWidth + 5, Y: textY, Button: tea.MouseLeft}))
	if cmd == nil {
		t.Fatal("expected copy command on input mouse release")
	}
	if updated.copyNotification != copyNotificationMessage {
		t.Fatalf("expected copy notification, got %q", updated.copyNotification)
	}
	if got := updated.inputSelection.selectedText(); got != "hello" {
		t.Fatalf("expected input selection to remain, got %q", got)
	}
	if strings.Contains(updated.selection.selectedText(), "hello") {
		t.Fatalf("expected viewport selection to stay clear, got %q", updated.selection.selectedText())
	}
	if !updated.textarea.Focused() {
		t.Fatal("expected input click to focus textarea")
	}
}

func TestReplInputSelectionCtrlCDoesNotCopySelection(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	m.inputSelection.setContent(m.textarea.Value())
	m.inputSelection.start(0, 0, 0)
	m.inputSelection.drag(5, 0, 0)

	updated, cmd := m.updateNormalMode(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("expected ctrl+c to clear input without copying")
	}
	if updated.quitting {
		t.Fatal("expected ctrl+c with input value not to quit")
	}
	if got := updated.textarea.Value(); got != "" {
		t.Fatalf("expected ctrl+c to clear input, got %q", got)
	}
}

func TestView_RendersInputSelection(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello world")
	m.inputSelection.setContent(m.textarea.Value())
	m.inputSelection.start(0, 0, 0)
	m.inputSelection.drag(5, 0, 0)

	view := m.View().Content
	plainView := ansi.Strip(view)
	if !strings.Contains(plainView, "hello") {
		t.Fatalf("expected input content in view, got %q", view)
	}
	withoutSelection := newTestModel()
	withoutSelection.textarea.SetValue("hello world")
	if view == withoutSelection.View().Content {
		t.Fatal("expected input selection to affect rendered view")
	}
}

func TestView_ShowsCopyNotification(t *testing.T) {
	m := newTestModel()
	m.copyNotification = copyNotificationMessage

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, copyNotificationMessage) {
		t.Fatalf("expected copy notification in view, got %q", view)
	}
}

func TestReplCopyNotificationExpires(t *testing.T) {
	m := newTestModel()
	expiresAt := time.Now().Add(copyNotificationTimeout)
	m.copyNotification = copyNotificationMessage
	m.copyNotificationExpiresAt = expiresAt

	updated, cmd := m.updateNormalMode(copyNotificationExpiredMsg{expiresAt: expiresAt.UnixNano()})
	if cmd != nil {
		t.Fatal("expected no command for copy notification expiry")
	}
	if updated.copyNotification != "" {
		t.Fatalf("expected copy notification to clear, got %q", updated.copyNotification)
	}
}

func TestReplCopyNotificationIgnoresStaleExpiry(t *testing.T) {
	m := newTestModel()
	expiresAt := time.Now().Add(copyNotificationTimeout)
	m.copyNotification = copyNotificationMessage
	m.copyNotificationExpiresAt = expiresAt

	updated, _ := m.updateNormalMode(copyNotificationExpiredMsg{expiresAt: expiresAt.Add(-time.Second).UnixNano()})
	if updated.copyNotification != copyNotificationMessage {
		t.Fatalf("expected copy notification to remain, got %q", updated.copyNotification)
	}
}

func TestView_KeepsAltScreenMouseCaptureForSelection(t *testing.T) {
	m := newTestModel()

	view := m.View()

	if !view.AltScreen {
		t.Fatal("expected alt-screen to remain enabled")
	}
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected cell motion mouse capture, got %v", view.MouseMode)
	}
}
