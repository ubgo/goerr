// Package goerr is a structured error type for application-level errors
// that carry user-facing copy, an error code, contextual key/value data,
// trace correlation IDs, and a severity level. It is designed to flow
// from your service layer to your HTTP / gRPC layer without lossy
// stringification.
//
// Two messages: the Error.Error() return is the operator-facing message
// (logs, alerts), the ErrorUser is the user-facing message (HTTP response
// body, GraphQL error). Use SetMessage / SetMessageUser, or the
// constructor options, to populate them independently.
//
// Global callbacks can be registered with AddCallback to forward every
// error to an external sink (Sentry, Loki, Slack). Callbacks fire on
// Error.Error() and ErrorUser.Error() so they capture every formatted
// surface.
package goerr

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Error is the structured error type. Construct with New / NewDefault
// and the With* options.
type Error struct {
	resp *ErrorResult
	ctx  context.Context //nolint:containedctx // ctx is intentional — see WithCtx
}

// Error returns the operator-facing message and fires registered callbacks.
func (e *Error) Error() string {
	runCallbacks(e)
	return e.resp.Message
}

// ErrorUser returns a separate sub-error that surfaces the user-facing message.
func (e *Error) ErrorUser() *ErrorUser {
	return &ErrorUser{resp: e.resp, err: e}
}

// Response returns a copy of the underlying ErrorResult.
func (e *Error) Response() ErrorResult {
	return *e.resp
}

// SetMessage updates the operator-facing message after construction.
func (e *Error) SetMessage(message string) *Error {
	e.resp.Message = message
	return e
}

// SetMessageUser updates the user-facing message after construction.
func (e *Error) SetMessageUser(message string) *Error {
	e.resp.MessageUser = message
	return e
}

// SetKV adds or overwrites a single contextual key/value pair.
func (e *Error) SetKV(key string, value any) *Error {
	if e.resp.KV == nil {
		e.resp.KV = make(map[string]any)
	}
	e.resp.KV[key] = value
	return e
}

// ErrorUser is a thin wrapper around Error whose .Error() returns the
// user-facing message. Use it when surfacing errors to clients (HTTP
// response bodies, GraphQL errors) so internal implementation details
// don't leak.
type ErrorUser struct {
	resp *ErrorResult
	err  *Error
}

// Error returns the user-facing message and fires registered callbacks.
func (e *ErrorUser) Error() string {
	runCallbacks(e.err)
	return e.resp.MessageUser
}

// ErrorResult is the wire-format payload of an error.
type ErrorResult struct {
	MessageUser string         `json:"messageUser,omitempty"`
	Message     string         `json:"message,omitempty"`
	Code        string         `json:"code,omitempty"`
	Data        any            `json:"data,omitempty"`
	TraceID     string         `json:"traceId,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Level       string         `json:"level,omitempty"`
	KV          map[string]any `json:"kv,omitempty"`
}

// ToJSON returns the ErrorResult marshalled as a JSON string. On marshal
// failure (which should be impossible for the documented field types) it
// returns the empty string.
func (e *ErrorResult) ToJSON() string {
	b, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	return string(b)
}

// New constructs an Error with the given operator-facing message. The user-
// facing message defaults to the same string; override with WithMessageUser.
func New(message string, options ...ErrorOption) *Error {
	resp := &ErrorResult{
		Message:     message,
		MessageUser: message,
		Timestamp:   time.Now().Format(time.RFC3339),
		Level:       "error",
		KV:          make(map[string]any),
	}
	e := &Error{resp: resp}
	for _, option := range options {
		option(e)
	}
	return e
}

// NewDefault is New with a generic "An error occurred" message. Use it as
// a starting point for a per-call site that injects all its data via
// SetMessage / SetMessageUser at error time.
func NewDefault(options ...ErrorOption) *Error {
	return New("An error occurred", options...)
}

// ErrorOption configures a New / NewDefault Error.
type ErrorOption func(*Error)

// WithMessageUser overrides the default user-facing message.
func WithMessageUser(message string) ErrorOption {
	return func(e *Error) { e.resp.MessageUser = message }
}

// WithCode tags the error with a stable code identifier (e.g. "E_NOT_FOUND").
func WithCode(code string) ErrorOption {
	return func(e *Error) { e.resp.Code = code }
}

// WithData attaches an arbitrary payload that will appear in ToJSON output.
func WithData(data any) ErrorOption {
	return func(e *Error) { e.resp.Data = data }
}

// WithTraceID sets the trace correlation ID. Typically populated from your
// tracing library.
func WithTraceID(traceID string) ErrorOption {
	return func(e *Error) { e.resp.TraceID = traceID }
}

// WithLevel overrides the default "error" severity.
func WithLevel(level string) ErrorOption {
	return func(e *Error) { e.resp.Level = level }
}

// WithKVMap adds every entry from kv to the error's KV map. Existing keys
// are overwritten.
func WithKVMap(kv map[string]any) ErrorOption {
	return func(e *Error) {
		if e.resp.KV == nil {
			e.resp.KV = make(map[string]any)
		}
		for k, v := range kv {
			e.resp.KV[k] = v
		}
	}
}

// WithKV adds a single key/value pair to the error's KV map.
func WithKV(key string, value any) ErrorOption {
	return func(e *Error) {
		if e.resp.KV == nil {
			e.resp.KV = make(map[string]any)
		}
		e.resp.KV[key] = value
	}
}

// WithCtx attaches a context.Context to the error. Callbacks receive this
// context, so observability hooks can extract trace IDs, request IDs, and
// the active logger from it.
func WithCtx(ctx context.Context) ErrorOption {
	return func(e *Error) { e.ctx = ctx }
}

// callbackFn is the global error-callback signature.
type callbackFn func(ctx context.Context, resp *ErrorResult)

var (
	callbackMu sync.RWMutex
	callbacks  []callbackFn
)

// AddCallback registers a global hook that runs on every Error.Error() and
// ErrorUser.Error() invocation. Callbacks fire synchronously in registration
// order — long-running observers should fan out to a goroutine themselves.
func AddCallback(callback callbackFn) {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	callbacks = append(callbacks, callback)
}

// ResetCallbacks removes all registered callbacks. Mainly useful for tests
// to keep callback state from leaking between table-driven cases.
func ResetCallbacks() {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	callbacks = nil
}

func runCallbacks(e *Error) {
	callbackMu.RLock()
	cbs := append([]callbackFn(nil), callbacks...)
	callbackMu.RUnlock()
	for _, cb := range cbs {
		cb(e.ctx, e.resp)
	}
}
