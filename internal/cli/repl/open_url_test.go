package repl

import (
	"os/exec"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestURLAtDisplayColumn(t *testing.T) {
	styled := "  See \x1b[4m\x1b]8;;https://example.com/path\x1b\\https://example.com/path\x1b]8;;\x1b\\\x1b[0m now."
	plain := ansi.Strip(styled)
	urlStart := indexOf(plain, "https://")

	tests := []struct {
		name string
		col  int
		want string
	}{
		{"start of url", urlStart, "https://example.com/path"},
		{"middle of url", urlStart + 5, "https://example.com/path"},
		{"before url", 0, ""},
		{"after url", urlStart + len("https://example.com/path") + 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := urlAtDisplayColumn(styled, tt.col); got != tt.want {
				t.Fatalf("urlAtDisplayColumn(col=%d) = %q, want %q", tt.col, got, tt.want)
			}
		})
	}
}

func TestURLAtDisplayColumnTrailingPunctuation(t *testing.T) {
	line := "visit https://example.com."
	start := indexOf(line, "https://")
	if got := urlAtDisplayColumn(line, start); got != "https://example.com" {
		t.Fatalf("expected trailing period stripped, got %q", got)
	}
	// The trailing period column should not be part of the URL.
	if got := urlAtDisplayColumn(line, len(line)-1); got != "" {
		t.Fatalf("expected no URL at trailing period, got %q", got)
	}
}

func TestURLAtDisplayColumnBackticks(t *testing.T) {
	line := "see `https://google.com` here"
	col := indexOf(line, "https://") + 3
	if got := urlAtDisplayColumn(line, col); got != "https://google.com" {
		t.Fatalf("expected backtick stripped, got %q", got)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestModifierClickOpensURL(t *testing.T) {
	var opened string
	orig := openURLCmd
	openURLCmd = func(url string) *exec.Cmd {
		opened = url
		return exec.Command("true")
	}
	t.Cleanup(func() { openURLCmd = orig })

	m := newTestModel()
	m.blurInput()
	m.selection.setContent("Visit https://example.com/docs here")

	col := indexOf("Visit https://example.com/docs here", "https://") + 3
	msg := tea.MouseClickMsg(tea.Mouse{X: col, Y: 0, Button: tea.MouseLeft, Mod: tea.ModAlt})
	handled, cmd := m.handleSelectionMouseDown(msg)
	if !handled {
		t.Fatal("expected modifier+click on URL to be handled")
	}
	if cmd == nil {
		t.Fatal("expected an open-url command")
	}
	cmd()
	if opened != "https://example.com/docs" {
		t.Fatalf("expected opened URL %q, got %q", "https://example.com/docs", opened)
	}
	if m.selection.hasSelection() {
		t.Fatal("expected no text selection after modifier+click")
	}
}

func TestPlainClickDoesNotOpenURL(t *testing.T) {
	var opened string
	orig := openURLCmd
	openURLCmd = func(url string) *exec.Cmd {
		opened = url
		return exec.Command("true")
	}
	t.Cleanup(func() { openURLCmd = orig })

	m := newTestModel()
	m.blurInput()
	m.selection.setContent("Visit https://example.com/docs here")

	col := indexOf("Visit https://example.com/docs here", "https://") + 3
	msg := tea.MouseClickMsg(tea.Mouse{X: col, Y: 0, Button: tea.MouseLeft})
	if _, cmd := m.handleSelectionMouseDown(msg); cmd != nil {
		t.Fatal("expected plain click not to open URL")
	}
	if opened != "" {
		t.Fatalf("expected no URL opened, got %q", opened)
	}
}
