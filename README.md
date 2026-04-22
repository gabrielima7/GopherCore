<p align="center">
  <h1 align="center">🦫 GopherCore</h1>
  <p align="center">
    <strong>Modular, secure, and scalable Go stack for robust development</strong>
  </p>
  <p align="center">
    Security guards · Result types · Retry logic · Circuit breakers · HTTP toolkit · Database kit · Async helpers
  </p>
  <p align="center">
    <a href="https://github.com/gabrielima7/GopherCore/actions/workflows/ci.yml"><img src="https://github.com/gabrielima7/GopherCore/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://pkg.go.dev/github.com/gabrielima7/GopherCore"><img src="https://pkg.go.dev/badge/github.com/gabrielima7/GopherCore.svg" alt="Go Reference"></a>
    <a href="https://goreportcard.com/report/github.com/gabrielima7/GopherCore"><img src="https://goreportcard.com/badge/github.com/gabrielima7/GopherCore" alt="Go Report Card"></a>
    <img src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go" alt="Go 1.26">
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="License">
  </p>
</p>

---

## Overview

GopherCore is a collection of production-ready Go packages designed to accelerate development while maintaining security and reliability. Each package is independently importable, well-tested, and follows Go idioms.

### Packages

| Package | Description |
|---------|-------------|
| [`result`](#result) | Generic `Result[T]` type for value-or-error handling |
| [`retry`](#retry) | Retry logic with exponential backoff and jitter |
| [`circuitbreaker`](#circuit-breaker) | Circuit breaker pattern for fault tolerance |
| [`guard`](#guard) | Input validation and sanitization |
| [`jsonutil`](#json-utilities) | Fast JSON encoding/decoding via `goccy/go-json` |
| [`httpkit`](#http-toolkit) | Chi router with security middleware, rate limiting, CORS |
| [`dbkit`](#database-toolkit) | Database connections via sqlx + migration management |
| [`async`](#async-helpers) | Safe goroutine management with panic recovery |
| [`logkit`](#structured-logs) | Structured logging in JSON format via `log/slog` |

---

## Installation

```bash
go get github.com/gabrielima7/GopherCore@latest
```

Import only the packages you need:

```go
import (
    "github.com/gabrielima7/GopherCore/result"
    "github.com/gabrielima7/GopherCore/retry"
    "github.com/gabrielima7/GopherCore/httpkit"
)
```

**Requirements:** Go 1.26+

---

## Usage

### Result

Generic `Result[T]` type that encapsulates either a value or an error. Compatible with Go's native error handling — no panics.

```go
import "github.com/gabrielima7/GopherCore/result"

// Create from Go's standard (value, error) pattern
r := result.Of(strconv.Atoi("42"))

if r.IsOk() {
    val, _ := r.Unwrap()
    fmt.Println(val) // 42
}

// Chain operations
doubled := result.Map(r, func(v int) int { return v * 2 })

// Provide fallbacks
val := r.UnwrapOr(0)

// Chain fallible operations
parsed := result.FlatMap(r, func(v int) result.Result[string] {
    if v < 0 {
        return result.Err[string](errors.New("negative"))
    }
    return result.Ok(fmt.Sprintf("value: %d", v))
})
```

### Retry

Retry operations with configurable backoff, jitter, and context support.

```go
import "github.com/gabrielima7/GopherCore/retry"

err := retry.Do(ctx, func(ctx context.Context) error {
    return callExternalAPI(ctx)
}, 
    retry.WithMaxAttempts(5),
    retry.WithInitialDelay(100*time.Millisecond),
    retry.WithStrategy(retry.StrategyExponential),
    retry.WithJitter(true),
    retry.WithRetryIf(func(err error) bool {
        return !errors.Is(err, ErrPermanent)
    }),
)

// With return value
val, err := retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
    return fetchData(ctx)
}, retry.WithMaxAttempts(3))
```

### Circuit Breaker

Prevent cascading failures with the circuit breaker pattern.

```go
import "github.com/gabrielima7/GopherCore/circuitbreaker"

cb := circuitbreaker.New(circuitbreaker.Config{
    FailureThreshold:    5,
    SuccessThreshold:    2,
    Timeout:             30 * time.Second,
    MaxHalfOpenRequests: 1,
    OnStateChange: func(from, to circuitbreaker.State) {
        log.Printf("circuit: %s → %s", from, to)
    },
})

err := cb.Execute(func() error {
    return callService()
})

if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
    // Circuit is open — service is likely down
}
```

### Guard

Input validation using `go-playground/validator` with structured errors and input sanitization.

```go
import "github.com/gabrielima7/GopherCore/guard"

type CreateUserInput struct {
    Name  string `validate:"required,min=2,max=100"`
    Email string `validate:"required,email"`
    Age   int    `validate:"gte=0,lte=150"`
}

input := CreateUserInput{Name: "A", Email: "invalid", Age: -1}
if err := guard.Validate(input); err != nil {
    var ve guard.ValidationErrors
    if errors.As(err, &ve) {
        for _, e := range ve {
            fmt.Printf("%s: %s\n", e.Field, e.Message)
        }
    }
}

// Sanitization
clean := guard.SanitizeString("hello\x00world")  // "helloworld"
text := guard.StripHTML("<script>alert(1)</script>") // "alert(1)"
```

### JSON Utilities

Fast JSON encoding/decoding, API-compatible with `encoding/json`.

```go
import "github.com/gabrielima7/GopherCore/jsonutil"

data, _ := jsonutil.Marshal(myStruct)
_ = jsonutil.Unmarshal(data, &result)

// Streaming
enc := jsonutil.NewEncoder(w)
dec := jsonutil.NewDecoder(r)
```

### HTTP Toolkit

Chi-based router with security middleware pre-configured.

```go
import "github.com/gabrielima7/GopherCore/httpkit"

r := httpkit.NewRouter(
    httpkit.WithCORS("https://example.com"),
    httpkit.WithRateLimit(100, 200),
    httpkit.WithLogger(true),
)

r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
    users := fetchUsers()
    httpkit.Ok(w, users)
})

r.Post("/api/users", func(w http.ResponseWriter, r *http.Request) {
    // ...
    httpkit.Created(w, newUser)
})

srv := httpkit.NewServer(":8080", r)
log.Fatal(srv.ListenAndServe())
```

**Built-in security headers** (always enabled):
- `Strict-Transport-Security` (HSTS with preload)
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Content-Security-Policy: default-src 'self'`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `X-XSS-Protection: 1; mode=block`
- `Permissions-Policy`

### Database Toolkit

Secure database connections with sqlx and golang-migrate integration.

```go
import "github.com/gabrielima7/GopherCore/dbkit"

db, err := dbkit.Connect(ctx, "postgres", dsn,
    dbkit.WithMaxOpenConns(25),
    dbkit.WithMaxIdleConns(5),
)
if err != nil {
    log.Fatal(err)
}

// Run migrations
err = dbkit.RunMigrations(db, "postgres", pgDriver, "file://./migrations")

// Health check
err = dbkit.HealthCheck(ctx, db)
```

### Structured Logs

A simple configuration using Go's native `log/slog` to output application logs in JSON format, making them easier to ingest by platforms like Datadog or Elasticsearch.

```go
import "github.com/gabrielima7/GopherCore/logkit"
import "log/slog"

// Initialize the global logger
logkit.Initialize(
    logkit.WithLevel(slog.LevelInfo),
)

slog.Info("user logged in", "user_id", 123, "ip", "192.168.1.1")
// Output: {"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"user logged in","user_id":123,"ip":"192.168.1.1"}

// Or create a local logger
logger := logkit.NewLogger(logkit.WithLevel(slog.LevelDebug))
logger.Debug("debugging request")
```

### Async Helpers

Safe goroutine management with panic recovery.

```go
import "github.com/gabrielima7/GopherCore/async"

// Safe goroutine with panic recovery
async.Go(func() {
    riskyOperation()
}, func(err error) {
    log.Printf("recovered: %v", err)
})

// Group of goroutines
g := async.NewGroup()
g.Go(func() error { return task1() })
g.Go(func() error { return task2() })
errs := g.Wait()

// Bounded concurrent map
results, err := async.Map(ctx, items, 10, func(ctx context.Context, item Item) (Result, error) {
    return process(ctx, item)
})
```

---

## Development

### Prerequisites

- Go 1.26+
- [golangci-lint](https://golangci-lint.run/) v2.11+
- [NilAway](https://github.com/uber-go/nilaway) (static nil-panic prevention)
- [gosec](https://github.com/securego/gosec)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)

### Install Tools

```bash
make install-tools
```

### Commands

```bash
make lint        # Run golangci-lint + NilAway
make nilaway     # Run NilAway static nil analysis
make test        # Run tests with coverage and race detector
make fuzz        # Run fuzz tests (30s per target)
make security    # Run gosec security analysis
make vulncheck   # Run govulncheck dependency audit
make audit       # Run ALL checks
make tidy        # Tidy go.mod
make fmt         # Format code
```

---

## CI/CD

The project uses GitHub Actions with a rigorous pipeline:

| Job | Platform | Description |
|-----|----------|-------------|
| **Lint** | Ubuntu | golangci-lint + NilAway static nil analysis |
| **Test** | Ubuntu, macOS, Windows | Tests with `-cover -race` |
| **Fuzz** | Ubuntu | 60s fuzz testing per package |
| **Security** | Ubuntu | gosec with SARIF → GitHub Security |
| **Vulncheck** | Ubuntu | govulncheck for dependency vulnerabilities |

**Dependabot** is configured for daily Go module updates and weekly GitHub Actions updates.

---

## Security

- **Input validation** via `go-playground/validator` (guard package)
- **SQL injection prevention** via prepared statements (`database/sql` + `sqlx`)
- **Security headers** (HSTS, CSP, X-Frame-Options, etc.)
- **Rate limiting** via `golang.org/x/time/rate`
- **CORS control** with configurable origins
- **Static analysis** with gosec in CI/CD
- **Static nil dereference analysis** with NilAway in CI/CD
- **Dependency scanning** with govulncheck
- **Linting** with golangci-lint (errcheck, nilerr, staticcheck)

---

## License

MIT License — see [LICENSE](LICENSE) for details.
