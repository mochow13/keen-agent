package mcp

import (
	"errors"
	"fmt"
)

var (
	ErrServerNotConfigured = errors.New("mcp server not configured")
	ErrServerDisconnected  = errors.New("mcp server disconnected")
	ErrAuthRequired        = errors.New("mcp authentication required")
	ErrAuthFailed          = errors.New("mcp authentication failed")
	ErrToolNotFound        = errors.New("mcp tool not found")
	ErrTimeout             = errors.New("mcp operation timed out")
	ErrProtocol            = errors.New("mcp protocol error")
	ErrRemoteTool          = errors.New("mcp remote tool error")
	ErrAlreadyStarted      = errors.New("mcp manager already started")
)

type Error struct {
	Kind   error
	Server string
	Tool   string
	Err    error
}

func (e *Error) Error() string {
	msg := e.Kind.Error()
	if e.Server != "" {
		msg += ": " + e.Server
	}
	if e.Tool != "" {
		msg += "/" + e.Tool
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *Error) Unwrap() error {
	if e.Err == nil {
		return e.Kind
	}
	return errors.Join(e.Kind, e.Err)
}

func newError(kind error, server, tool string, err error) error {
	return &Error{Kind: kind, Server: server, Tool: tool, Err: err}
}

func stateError(name string, state ServerState, lastErr string) error {
	switch state {
	case StateAuthRequired:
		return newError(ErrAuthRequired, name, "", errors.New(lastErr))
	case StateAuthFailed:
		return newError(ErrAuthFailed, name, "", errors.New(lastErr))
	default:
		if lastErr == "" {
			return newError(ErrServerDisconnected, name, "", nil)
		}
		return newError(ErrServerDisconnected, name, "", fmt.Errorf("%s", lastErr))
	}
}
