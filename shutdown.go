package singleserve

import (
	"context"
	"regexp"
)

// ShutdownReason records why graceful shutdown began.
type ShutdownReason string

const (
	ShutdownBrowserRequest      ShutdownReason = "browser_request"
	ShutdownBrowserDisconnected ShutdownReason = "browser_disconnected"
	ShutdownFirstContactTimeout ShutdownReason = "first_contact_timeout"
	ShutdownProgrammatic        ShutdownReason = "programmatic"
	ShutdownContextCanceled     ShutdownReason = "context_canceled"
	ShutdownServerError         ShutdownReason = "server_error"
)

// ShutdownRequest is the server-built input to an application shutdown guard.
type ShutdownRequest struct {
	Reason ShutdownReason
	Tabs   []TabSnapshot
}

// ShutdownGuard can veto browser-initiated or browser-bound shutdown.
type ShutdownGuard interface {
	CheckShutdown(context.Context, ShutdownRequest) error
}

// ShutdownGuardFunc adapts a function to ShutdownGuard.
type ShutdownGuardFunc func(context.Context, ShutdownRequest) error

// CheckShutdown calls f.
func (f ShutdownGuardFunc) CheckShutdown(ctx context.Context, request ShutdownRequest) error {
	if f == nil {
		return nil
	}
	return f(ctx, request)
}

var shutdownCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ShutdownDeniedError is a safe, expected application veto.
type ShutdownDeniedError struct {
	Code    string
	Message string
}

func (e *ShutdownDeniedError) Error() string {
	if e == nil {
		return "shutdown denied"
	}
	return e.Message
}

func (e *ShutdownDeniedError) valid() bool {
	return e != nil && shutdownCodePattern.MatchString(e.Code) && e.Message != ""
}

// DenyShutdown creates a typed shutdown veto. Invalid codes or empty messages
// are treated as internal guard failures when evaluated.
func DenyShutdown(code, message string) error {
	return &ShutdownDeniedError{Code: code, Message: message}
}
