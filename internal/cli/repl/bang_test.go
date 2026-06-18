package repl

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestStartBangCommand_StreamsStdoutStderrAndDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	messages := collectBangMessages(t, startBangCommand(ctx, `printf 'out\n'; printf 'err\n' >&2`))

	var gotStdout, gotStderr, gotDone bool
	for _, msg := range messages {
		switch msg := msg.(type) {
		case bangOutputMsg:
			if msg.stream == bangStreamStdout && msg.line == "out" {
				gotStdout = true
			}
			if msg.stream == bangStreamStderr && msg.line == "err" {
				gotStderr = true
			}
		case bangDoneMsg:
			gotDone = true
			if msg.err != nil {
				t.Fatalf("expected successful done message, got %v", msg.err)
			}
		}
	}

	if !gotStdout {
		t.Fatal("expected stdout line")
	}
	if !gotStderr {
		t.Fatal("expected stderr line")
	}
	if !gotDone {
		t.Fatal("expected done message")
	}
}

func TestStartBangCommand_ReportsExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := lastBangDoneMsg(t, startBangCommand(ctx, "exit 7"))
	if done.err == nil {
		t.Fatal("expected exit error")
	}
	if done.exitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", done.exitCode)
	}
	if done.timedOut {
		t.Fatal("did not expect timeout")
	}
}

func TestStartBangCommand_ReportsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	done := lastBangDoneMsg(t, startBangCommand(ctx, "sleep 1"))
	if !done.timedOut {
		t.Fatal("expected timeout")
	}
	if done.err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestWaitForBangEvent_NilChannelReturnsNil(t *testing.T) {
	if cmd := waitForBangEvent(nil); cmd != nil {
		t.Fatal("expected nil command for nil event channel")
	}
}

func TestHandleBangMsg_StreamsOutputAndFinishes(t *testing.T) {
	m := newTestModel()
	events := make(chan tea.Msg)
	m.bang = bangState{active: true, events: events}

	newM, cmd, handled := m.handleBangMsg(bangOutputMsg{stream: bangStreamStdout, line: "hello"})
	if !handled {
		t.Fatal("expected stdout message to be handled")
	}
	if cmd == nil {
		t.Fatal("expected next wait command")
	}
	if !strings.Contains(newM.output.Join(), "hello") {
		t.Fatal("expected stdout line in output")
	}

	newM, _, handled = newM.handleBangMsg(bangOutputMsg{stream: bangStreamStderr, line: "warn"})
	if !handled {
		t.Fatal("expected stderr message to be handled")
	}
	if !strings.Contains(newM.output.Join(), "warn") {
		t.Fatal("expected stderr line in output")
	}

	newM, cmd, handled = newM.handleBangMsg(bangDoneMsg{duration: time.Millisecond})
	if !handled {
		t.Fatal("expected done message to be handled")
	}
	if cmd != nil {
		t.Fatal("expected no next command after done")
	}
	if newM.bang.active {
		t.Fatal("expected bang command to be inactive")
	}
	if !strings.Contains(newM.output.Join(), "done in 1ms") {
		t.Fatal("expected completion summary")
	}
}

func TestCancelBangCommand_ClearsCancelFunc(t *testing.T) {
	m := newTestModel()
	called := false
	m.bang = bangState{
		active: true,
		cancel: func() {
			called = true
		},
	}

	m.cancelBangCommand()

	if !called {
		t.Fatal("expected cancel function to be called")
	}
	if m.bang.cancel != nil {
		t.Fatal("expected cancel function to be cleared")
	}
}

func collectBangMessages(t *testing.T, events <-chan tea.Msg) []tea.Msg {
	t.Helper()

	var messages []tea.Msg
	timeout := time.After(2 * time.Second)
	for {
		select {
		case msg, ok := <-events:
			if !ok {
				return messages
			}
			messages = append(messages, msg)
		case <-timeout:
			t.Fatal("timed out waiting for bang messages")
		}
	}
}

func lastBangDoneMsg(t *testing.T, events <-chan tea.Msg) bangDoneMsg {
	t.Helper()

	var done bangDoneMsg
	found := false
	for _, msg := range collectBangMessages(t, events) {
		if doneMsg, ok := msg.(bangDoneMsg); ok {
			done = doneMsg
			found = true
		}
	}
	if !found {
		t.Fatal("expected bang done message")
	}
	return done
}
