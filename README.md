# goerr

A structured error type that carries an operator-facing message, a separate user-facing message, an error code, contextual key/value data, a trace correlation ID, and a severity level — so errors flow from your service layer to your HTTP / gRPC layer without lossy stringification.

Zero third-party dependencies.

## Install

```sh
go get github.com/ubgo/goerr
```

## Quick start

```go
package main

import (
    "fmt"

    "github.com/ubgo/goerr"
)

func main() {
    err := goerr.New("db: deadlock detected on user_orders",
        goerr.WithMessageUser("Something went wrong, please retry"),
        goerr.WithCode("E_TX_RETRY"),
        goerr.WithTraceID("trace-abc123"),
        goerr.WithKV("table", "user_orders"),
    )

    fmt.Println(err.Error())             // operator-facing
    fmt.Println(err.ErrorUser().Error()) // user-facing
    fmt.Println(err.Response().ToJSON()) // wire payload
}
```

Output:

```
db: deadlock detected on user_orders
Something went wrong, please retry
{"messageUser":"Something went wrong, please retry","message":"db: deadlock detected on user_orders","code":"E_TX_RETRY","traceId":"trace-abc123","timestamp":"2026-05-02T10:11:12Z","level":"error","kv":{"table":"user_orders"}}
```

## Two messages — why?

Production errors usually need two surfaces:

| Surface | Audience | Example |
|---------|----------|---------|
| `Error()` (operator) | Logs, alerts, on-call dashboards | `db: deadlock detected on user_orders pk=42 retried=3` |
| `ErrorUser()` (user) | HTTP body, GraphQL error response | `Service is busy, please try again in a moment` |

Mixing them up leaks internals to users or hides actionable detail from operators. `goerr` keeps them on the same struct so you can construct once and surface twice.

## Global callbacks

Forward every error to an external sink (Sentry, Loki, Slack) without sprinkling reporter calls everywhere:

```go
goerr.AddCallback(func(ctx context.Context, r *goerr.ErrorResult) {
    sentryClient.CaptureException(ctx, r)
})
```

Callbacks fire synchronously on `Error.Error()` and `ErrorUser.Error()` calls. Long-running observers should fan out to a goroutine themselves.

For tests, `goerr.ResetCallbacks()` clears all registered hooks.

## API

### Constructors

| Function | Purpose |
|----------|---------|
| `New(message string, options ...ErrorOption) *Error` | Operator + user message default to `message`. |
| `NewDefault(options ...ErrorOption) *Error` | Generic "An error occurred" message; intended as a starting point you mutate per call site. |

### Options

| Option | Purpose |
|--------|---------|
| `WithMessageUser(string)` | Override the user-facing message. |
| `WithCode(string)` | Stable error code (e.g. `E_NOT_FOUND`). |
| `WithData(any)` | Arbitrary payload visible in `ToJSON`. |
| `WithTraceID(string)` | Trace correlation ID. |
| `WithLevel(string)` | Severity (default `"error"`). |
| `WithKV(key, value)` / `WithKVMap(map)` | Add contextual key/value entries. |
| `WithCtx(context.Context)` | Attach a context — passed to callbacks. |

### Methods

| Method | Purpose |
|--------|---------|
| `(*Error).Error() string` | Operator-facing message, fires callbacks. |
| `(*Error).ErrorUser() *ErrorUser` | Wrapped error whose `.Error()` returns the user-facing message. |
| `(*Error).Response() ErrorResult` | Copy of the wire payload. |
| `(*Error).SetMessage / SetMessageUser / SetKV` | Mutate after construction. |
| `(*ErrorResult).ToJSON() string` | Marshal to JSON for HTTP responses. |
| `AddCallback(fn)` / `ResetCallbacks()` | Manage global error hooks. |

## License

Apache License 2.0. See [`LICENSE`](./LICENSE).
