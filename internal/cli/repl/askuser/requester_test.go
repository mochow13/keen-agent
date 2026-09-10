package askuser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mochow13/keen-agent/internal/tools"
)

func TestRequester_CancellationClearsPendingRequest(t *testing.T) {
	requester := NewRequester()
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := requester.RequestUser(ctx, tools.AskUserRequest{})
		resultCh <- err
	}()

	req := <-requester.GetRequestChan()
	cancel()
	if err := <-resultCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("RequestUser() error = %v, want context.Canceled", err)
	}
	if requester.IsPending(req) {
		t.Fatal("expected cancelled request to no longer be pending")
	}
}

func TestRequester_RejectsConcurrentQuestionnaire(t *testing.T) {
	requester := NewRequester()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	go func() {
		_, err := requester.RequestUser(ctx, tools.AskUserRequest{})
		firstDone <- err
	}()

	req := <-requester.GetRequestChan()
	_, err := requester.RequestUser(context.Background(), tools.AskUserRequest{})
	if err == nil {
		t.Fatal("expected concurrent request rejection")
	}
	requester.Respond(req, tools.AskUserResult{})
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first RequestUser() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first request did not complete")
	}
}

func TestRequester_RespondNilIsNoOp(t *testing.T) {
	requester := NewRequester()
	requester.Respond(nil, tools.AskUserResult{Answers: []string{"ignored"}})
}

func TestRequester_IsPendingNilIsFalse(t *testing.T) {
	requester := NewRequester()
	if requester.IsPending(nil) {
		t.Fatal("IsPending(nil) should be false")
	}
}
