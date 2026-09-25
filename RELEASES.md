# GopherCore Releases

This document tracks all major additions, alterations, deletions, and pull requests merged for each version of the GopherCore project.

## [v0.5.0] - Dual Go 1.26/1.27 Compatibility, Ultimate Chaos Verification, Context-Aware Circuit Breaking, and Concurrency Leak Hardening

This major release elevates GopherCore's production readiness, enterprise resilience, and formal verification. It introduces official dual-compatibility and continuous testing across Go 1.26+ and Go 1.27+ toolchains, expands the chaos engineering testing harness with the Ultimate Chaos Load Test scaling up to 10,000 concurrent goroutines alongside formal mathematical state convergence proofs, brings context-aware cancellation to circuit breaker execution, eliminates background goroutine leaks across all network and asynchronous worker components, and achieves 100% GoDoc Living Documentation compliance across all 14 packages.

### 🚀 Additions (Features & Enhancements)
- **Official Dual Go 1.26+ and Go 1.27+ Compatibility Matrix:** Configured CI/CD workflows (`ci.yml`, `test.yml`, `benchmark.yml`) with a dynamic test matrix targeting Go `1.26.x` and `1.27.x`. Added dynamic toolchain detection (`GOTOOLCHAIN ?= auto`) in the `Makefile` and conditional static analysis execution for NilAway. (PR #299)
- **Ultimate Chaos Load Test and Formal Verification:** Introduced `simulation/chaos_load_test.go` and empirical stress testing simulating up to 10,000 concurrent goroutines across caching, circuit breaking, retries, and HTTP services. Includes formal mathematical proofs demonstrating bounded memory allocation, zero deadlocks, and O(1) state convergence. (PR #291, PR #243)
- **Context-Aware Circuit Breaker:** Extended `circuitbreaker.Execute(ctx, fn)` with a first-class `context.Context` parameter, ensuring active cancellation propagation and fast-abort upon context deadline expiration to prevent goroutine exhaustion. (PR #275)
- **Asynq Worker Graceful Teardown:** Added strict graceful shutdown lifecycle hooks (`srv.Shutdown()`, `srv.Stop()`) to the `async` background worker implementation, ensuring zero dangling goroutines or unhandled queue worker terminations. (PR #217, PR #239)
- **Comprehensive Unhappy Path and TDT Test Suites:**
  - Expanded `cachekit` with edge-case tests covering nil keys, closed connections, empty payloads, and TTL overflows. (PR #292)
  - Added table-driven tests for nil function recovery in `async`. (PR #283)
  - Hardened `retry` and `guard` packages with zero-value, empty-options, and struct nil-pointer boundary tests. (PR #285, PR #249, PR #236)
  - Added dedicated unit tests for formatted error generation via `result.Errf`. (PR #264)
  - Added missing validation test for `dbkit.MustConnect` asserting panic on empty or malformed DSN. (PR #260)
  - Added tests asserting panic-free error responses on invalid HTTP status codes in `httpkit`. (PR #252)
  - Added Table-Driven Test coverage for `grpckit.NewClient` error paths and invalid configurations. (PR #199)
  - Validated `Result.Unwrap` panic contracts across exhaustive test matrices. (PR #231)

### 🛠 Changes (Modifications & Optimizations)
- **Goroutine Leak Hardening with `goleak`:** Integrated `go.uber.org/goleak` across all simulation and chaos test suites. Replaced shared `http.DefaultClient` instances with isolated test server clients (`srv.Client()`) and explicit `CloseIdleConnections()` invocations, eliminating lingering TCP keep-alive socket goroutines. (PR #298, PR #265, PR #263, PR #254, PR #245, PR #212)
- **JSON Error Response Content-Type Canonicalization:** Ensured `httpkit.Error()` consistently sets the `Content-Type: application/json` header before writing HTTP error payloads, guaranteeing compliant client-side deserialization. (PR #232)
- **NilAway Static Nil-Safety Remediation:** Added defensive nil checks to `net.Listener.Addr()` and network initialization paths, ensuring zero static nil dereference warnings under Go 1.26 NilAway analysis. (PR #299)
- **Graceful Shutdown Timeout Fallback:** Fixed an edge-case in `httpkit.GracefulShutdown` where supplying a zero duration resulted in instantaneous context expiration, implementing an automatic fallback to standard default timeout limits. (PR #210)
- **gRPC Custom Dialer Context Handling:** Fixed timeout deadline propagation in `grpckit` custom dialer logic to prevent premature connection timeouts or unmanaged background dials. (PR #206, PR #237)
- **Living Documentation & GoDoc Audits:** Conducted exhaustive repository-wide documentation synchronizations across all 14 packages, ensuring all exported interfaces, structs, functions, and internal mechanics include comprehensive `Internal Logic Deep-Dive` descriptions and contract specifications. (PR #296, PR #273, PR #272, PR #262, PR #259, PR #256, PR #251, PR #247, PR #242, PR #240, PR #227, PR #221, PR #218, PR #215, PR #202, PR #198)
- **Tooling & Dependency Upgrades:**
  - Upgraded `actions/setup-go` from v5 to v7 across CI workflows. (PR #203)
  - Upgraded `github/codeql-action` to `v4.38.0`. (PR #234, PR #241, PR #250, PR #258, PR #276)
  - Upgraded `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` to `v0.71.0`. (PR #246, PR #293)
  - Upgraded `google.golang.org/grpc` to `v1.84.0`. (PR #200, PR #235, PR #293)
  - Upgraded `github.com/redis/go-redis/v9` to `v9.22.0`. (PR #238, PR #293)
  - Upgraded `github.com/golang-migrate/migrate/v4` to `v4.20.1`. (PR #277)
  - Upgraded `github.com/mattn/go-sqlite3` to `v1.14.52`. (PR #235, PR #271, PR #293)
  - Upgraded `github.com/prometheus/client_golang` to `v1.24.1`. (PR #216)
  - Upgraded `go-sql-driver/mysql`, `go.uber.org/atomic`, and `google.golang.org/genproto` APIs. (PR #297, PR #204, PR #225, PR #280)

### 🗑️ Exclusions (Deprecations & Removals)
- **Removal of Static Go Toolchain Pinning:** Removed static `GOTOOLCHAIN=go1.26.6` declarations across developer build environments to allow seamless developer compilation under Go 1.26, Go 1.27, and future toolchains. (PR #299)
- **Removal of Leaking Default Transports in Tests:** Eliminated usage of shared HTTP default transports within integration tests to remove lingering background keep-alive socket goroutines. (PR #298)
- **Deprecation of Context-Free Circuit Breaking:** Deprecated invoking circuit-protected operations without context propagation; callers are directed to the context-aware `circuitbreaker.Execute(ctx, fn)`. (PR #275)

### 📦 Pull Requests
- **PR #299:** chore(build): official compatibility and support for Go 1.26+ and Go 1.27+
- **PR #298:** fix: eliminate testing goroutine leaks in chaos simulation
- **PR #297:** chore(deps): update mysql, atomic, and genproto dependencies
- **PR #296:** docs: perform exhaustive living documentation audit
- **PR #293:** build(deps): update core dependencies
- **PR #292:** test(cachekit): improve test robustness with unhappy path edge cases
- **PR #291:** feat(simulation): Ultimate Chaos Load Test and Formal Verification
- **PR #285:** test(gophercore): add edge case tests for retry and guard packages
- **PR #283:** test(async): add table-driven tests for nil function recovery
- **PR #280:** Update dependencies and clean go.mod/go.sum
- **PR #277:** Update dependency github.com/golang-migrate/migrate/v4 to v4.20.1
- **PR #276:** chore(deps): bump github/codeql-action from 4.37.9 to 4.38.0 in the actions-minor-patch group
- **PR #275:** feat(circuitbreaker): add context parameter to Execute
- **PR #273:** docs: Add missing internal logic deep-dives for interfaces
- **PR #272:** docs: add deep-dive comment to simulation package
- **PR #271:** chore(deps): bump the go-minor-patch group across 1 directory with 2 updates
- **PR #268:** Improve test coverage for httpkit and circuitbreaker
- **PR #267:** chore(deps): bump the go-minor-patch group with 2 updates
- **PR #265:** Fix httpkit panics and simulation goroutine leaks
- **PR #264:** test: add test coverage for result.Errf
- **PR #263:** Fix goroutine leaks in grpckit and simulation tests
- **PR #262:** docs: update package documentation descriptions
- **PR #261:** chore(deps): bump the go-minor-patch group with 8 updates
- **PR #260:** test(dbkit): add missing test for MustConnect empty DSN panic
- **PR #259:** docs: exhaustive documentation synchronization for gophercore
- **PR #258:** chore(deps): bump github/codeql-action from 4.37.7 to 4.37.9 in the actions-minor-patch group
- **PR #256:** chore: fix tdt tests and deep-dive documentation
- **PR #255:** chore(deps): bump the go-minor-patch group with 3 updates
- **PR #254:** test(simulation): fix goroutine leaks in chaos fuzz tests
- **PR #252:** test(httpkit): add tests for invalid status code panics
- **PR #251:** docs: add internal logic deep dive comments to exhaustive list of packages
- **PR #250:** chore(deps): bump github/codeql-action from 4.37.6 to 4.37.7 in the actions-minor-patch group
- **PR #249:** Add edge case and boundary unit tests for core utility packages
- **PR #247:** Update package docs
- **PR #246:** chore(deps): bump go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc from 0.69.0 to 0.70.0 in the go-minor-patch group
- **PR #245:** fix: add goroutine leak check to TestMassiveConcurrencyLoad
- **PR #243:** test(simulation): execute empirical simulation destruction and proof
- **PR #242:** chore: exhaustive full-project documentation audit and logic deep-dives
- **PR #241:** chore(deps): bump github/codeql-action from 4.37.3 to 4.37.6 in the actions-minor-patch group
- **PR #240:** docs: add internal logic deep-dive comments
- **PR #239:** fix(async): ensure graceful shutdown of Asynq workers to prevent goroutine leaks
- **PR #238:** chore: update go-redis and opentelemetry dependencies
- **PR #237:** fix: update grpckit custom dialer context timeout handling
- **PR #236:** test: improve coverage for guard and httpkit edge cases
- **PR #235:** Bump sqlite3, grpc and otelhttp dependencies
- **PR #234:** chore(deps): bump github/codeql-action from 3 to 4.37.3
- **PR #233:** chore(deps): bump the go-minor-patch group across 1 directory with 2 updates
- **PR #232:** Fix: Set JSON Content-Type correctly in httpkit error responses
- **PR #231:** test(result): cover Result.Unwrap with table-driven tests
- **PR #230:** chore: intelligently manage and update dependencies
- **PR #227:** chore: add rigorous documentation to internal unexported entities
- **PR #225:** chore(deps): update otelhttp and grpcgcp dependencies
- **PR #223:** build(deps): intelligently update outdated module dependencies
- **PR #221:** chore(docs): audit and verify living documentation
- **PR #218:** docs: Project-wide Exhaustive Documentation Audit
- **PR #217:** fix: gracefully shutdown asynq server to prevent goroutine leaks
- **PR #216:** chore(deps): bump github.com/prometheus/client_golang from 1.24.0 to 1.24.1 in the go-minor-patch group
- **PR #215:** Audit and verify GoDoc synchronization project-wide
- **PR #212:** test: empirical proof of no goroutine leaks via goleak and edge-case testing
- **PR #210:** fix: resolve instant context expiration in GracefulShutdown with zero timeout
- **PR #206:** Fix gRPC client context timeout bug
- **PR #204:** Update google genproto APIs
- **PR #203:** chore(deps): bump actions/setup-go from 5 to 7
- **PR #202:** docs: Exhaustive GoDoc documentation synchronization
- **PR #201:** Fix gosec test compilation for otelkit
- **PR #200:** chore(deps): bump google.golang.org/grpc from 1.82.0 to 1.82.1 in the go-minor-patch group
- **PR #199:** test(grpckit): add TDT coverage for NewClient error paths
- **PR #198:** docs: exhaustively fix all godoc comments

---

## [v0.4.1] - gRPC Client Hardening, HTTP Canonicalization, Extreme Concurrency Simulation, and QA Defensive Teardowns

This release improves the stability and compliance of GopherCore's networking and concurrency models. It migrates deprecated gRPC dial connections to the modern client initializer while preserving timeout behavior via custom context dialers. In addition, it enforces standard HTTP header canonicalization in all middleware and utility packages, introduces extreme concurrency simulations to prove O(1) space and time complexity convergence, and hardens test suites with defensive teardown logic to prevent G104 unhandled error violations.

### 🚀 Additions (Features & Enhancements)
- **Extreme Concurrency Chaos Simulations:** Introduced `simulation/chaos_extreme_test.go` and `simulation/chaos_load_test.go` to test API ergonomics, concurrency chaos, and stress-test the cache, circuit breaker, retry, result, and HTTP middleware packages under massive concurrency load (up to 5000 goroutines) with zero data races or goroutine leaks. (PR #172, PR #192)
- **Comprehensive gRPC Client Tests:** Added `grpckit/client_test.go` to test gRPC client options (`WithInsecure`, `WithClientTLS`, `WithDialTimeout`, `WithClientUnaryInterceptors`, `WithClientStreamInterceptors`, `WithRawDialOptions`), configuration parsing, and NewClient initialization logic, raising statement coverage to 100%. (PR #188)
- **Variadic Option Safety Tests:** Added `TestQueueClientEnqueueOptions` to `async/queue_test.go` to validate variadic `asynq.Option` parsing logic and nil-slice safety for `async.QueueClient` operations. (PR #189)

### 🛠 Changes (Modifications & Optimizations)
- **gRPC Client Initializer Migration:** Migrated the deprecated `grpc.DialContext` to the newer `grpc.NewClient` API. To preserve background connection timeout behavior, implemented a custom context dialer using `grpc.WithContextDialer` and `net.Dialer`. (PR #196)
- **HTTP Header Canonicalization:** Replaced direct header map writes (e.g. `h := w.Header(); h["Key"] = ...`) with standard `w.Header().Set(...)` assignments across `CORSMiddleware`, `RateLimitMiddleware`, `SecurityHeadersMiddleware`, and `JSON(...)` responses to ensure key canonicalization and avoid potential header collision overhead. (PR #193)
- **Defensive Test Teardowns (G104 Mitigation):** Wrapped test teardowns (such as `client.Close()`, `db.Close()`, `cache.Close()`) in deferred closures using blank identifier assignments (`_ = x.Close()`) to resolve `gosec G104` unhandled error findings across all package test suites. (PR #185, PR #197)
- **TDT Edge Cases and Safety checks:** Added zero-options and nil-pointer edge case tests to the `guard` and `retry` packages, including "no options provided" for `TestDo_TableDriven` and "nil pointer to struct" for `TestValidate_TableDriven`. (PR #179, PR #195)
- **Fuzz Input Clamping:** Refactored `simulation/chaos_fuzz_test.go` to clamp fuzz inputs via modulo operations instead of skipping negative inputs, increasing testing coverage for extreme bounds. (PR #187)
- **Dependency Upgrades:** Updated dependencies across the project:
    - Updated `go-chi/chi/v5` to `v5.3.1` and `golang.org/x/text` to `v0.39.0` (PR #176).
    - Updated `golang.org/x/crypto` to `v0.54.0` and `golang.org/x/net` to `v0.57.0` (PR #182, PR #186).
    - Updated `github.com/mattn/go-sqlite3` to `v1.14.48` (PR #190).
    - Updated indirect dependencies `google.golang.org/genproto/googleapis/api` and `rpc` to resolve potential security fixes (PR #194).
    - Updated indirect dependencies `github.com/klauspost/compress` to `v1.19.0` and `github.com/klauspost/cpuid/v2` to `v2.4.0` (PR #171).

### 🗑️ Exclusions (Deprecations & Removals)
- **Removal of Inapplicable Dial Tests:** Removed `TestNewClient_DialError` from `grpckit/grpckit_test.go` because the new `grpc.NewClient` API does not support the blocking `grpc.WithBlock` dial option, making synchronous connection failure tests obsolete. (PR #196)
- **Cleanup of Untracked Analysis Artifacts:** Deleted the untracked `escape_analysis.txt` file from the simulation directory to maintain a clean workspace. (PR #178)

### 📦 Pull Requests
- **PR #197:** fix(test): explicitly handle and ignore test teardown errors to resolve gosec G104.
- **PR #196:** fix(grpckit): migrate deprecated `grpc.DialContext` to `grpc.NewClient` and enforce dial timeout via custom net dialer.
- **PR #195:** test: improve test coverage for guard and retry packages by adding edge cases for zero options and nil structs.
- **PR #194:** chore(deps): update google.golang.org/genproto/googleapis/api and rpc to latest.
- **PR #193:** refactor(httpkit): use w.Header().Set() for canonicalization instead of direct Header map assignments.
- **PR #192:** test(simulation): add extreme concurrency chaos testing with 5000 concurrent goroutines.
- **PR #190:** chore(deps): bump github.com/mattn/go-sqlite3 from 1.14.47 to 1.14.48 in the go-minor-patch group.
- **PR #189:** test(async): add nil options safety tests for queue client.
- **PR #188:** test: add comprehensive unit tests for grpckit client (100% coverage).
- **PR #187:** test(simulation): fix FuzzChaos to safely clamp negative inputs without t.Skip().
- **PR #186:** build(deps): update x/net and prometheus/common dependencies.
- **PR #185:** fix(grpckit): handle unhandled conn.Close() errors in grpckit tests.
- **PR #184:** test: add targeted context cancellation test for async.Map.
- **PR #182:** build(deps): bump golang.org/x/crypto to v0.54.0 and related x/sys, x/text packages.
- **PR #179:** test: add comprehensive edge-case coverage for core utility options (dbkit, grpckit, httpkit).
- **PR #178:** fix(simulation): remove escape_analysis.txt and fix expected error filtering in chaos test.
- **PR #176:** build: update go-chi/chi and x/text dependencies.
- **PR #175:** test(httpkit): add coverage for RateLimitMiddleware bypass and nil limiter.
- **PR #174:** test: conduct rigorous chaos simulation and proofing.
- **PR #173:** chore: perform exhaustive living documentation audit.
- **PR #172:** test: implement chaos engineering simulations and formal proofs.
- **PR #171:** build(deps): update indirect dependencies compress to v1.19.0 and cpuid to v2.4.0.

---

## [v0.4.0] - Chaos Engineering, Concurrency Hardening, Escape Analysis, and Extensive TDT Coverage

This release hardens GopherCore's concurrency model, memory optimization, and test coverage. It implements heap escape analysis fixes to optimize allocations, resolves critical memory/goroutine leaks in active timers and caching structures, and expands the chaos engineering simulation suite with fuzz and integration tests. Additionally, the release refactors several core packages to Table-Driven Test (TDT) patterns to fix global state leakage and achieve 100% GoDoc living documentation compliance.

### 🚀 Additions (Features & Enhancements)
- **Chaos Engineering & Fuzzing Expansion:** Added robust chaos integration tests (`simulation/chaos_integration_test.go`) and expanded fuzz testing to the `circuitbreaker` and `retry` packages. Added formal verification, chaos fuzzing, and stress amplification (`simulation/chaos_fuzz_test.go`). (PR #123, PR #145, PR #158)
- **InMemory Cache Unit Tests:** Added `cachekit/redis_error_test.go` to validate error paths and unit tests for cache options. (PR #108)
- **Otelkit Coverage Improvements:** Improved unit test coverage for `otelkit` package initialization error paths. (PR #108, PR #127)

### 🛠 Changes (Modifications & Optimizations)
- **Escape Analysis and Stack Allocation:** Prevented `retry.Config` from escaping to the heap, implementing a stack-allocation fast-path for zero-option calls to minimize GC pressure. (PR #160)
- **Timer and Goroutine Leak Fixes:** Fixed a goroutine leak in `InMemoryCache` upon calling `Close()`, and eliminated timer-based memory leaks in the `retry` package and test simulations by explicitly calling `Stop()`. (PR #128, PR #130, PR #132)
- **Table-Driven Test (TDT) Refactoring:**
    - Refactored `dbkit` healthcheck tests into Table-Driven Test suites with context cancellation edge-case validation. (PR #159)
    - Refactored `logkit` tests into TDT suites, resolving global state leakage. (PR #166)
    - Refactored `jsonutil` tests to include robust table-driven validation for edge cases. (PR #149)
    - Converted `TestNewRouterWithMetricsPath` to TDT and added empty path validations. (PR #126)
- **Resilience and Error Assertions:** Resolved panic recovery tests and unhandled defer statements in `grpckit` tests, and enforced strict error handling and assertions in chaos microservice tests. (PR #119, PR #167)
- **Exhaustive Living Documentation Audit:** Completed a repository-wide docstring audit to ensure 100% GoDoc compliance, adding missing tags, correcting misplaced docstrings, formatting package comments, and updating mathematical state convergence proofs. (PR #113, PR #115, PR #135, PR #137, PR #141, PR #151)
- **CI/CD Pipeline Hardening:** Bumped `actions/checkout` version to `v7`, updated the project-wide Go version and GOTOOLCHAIN to `1.26.4` to patch standard library vulnerability checks, and updated the Makefile to run multiple fuzz tests sequentially. (PR #110, PR #123, PR #146)
- **Dependency Upgrades:**
    - Updated `google.golang.org/grpc` to `v1.82.0`. (PR #165)
    - Updated `github.com/mattn/go-sqlite3` to `v1.14.46`. (PR #140)
    - Updated `github.com/prometheus/common` to `v0.69.0`. (PR #138)
    - Updated `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` to `v0.69.0`. (PR #136)
    - Updated `github.com/redis/go-redis/v9` to `v9.20.1`. (PR #129)
    - Updated standard libraries `golang.org/x/crypto` and `golang.org/x/net`. (PR #131)
    - Updated `github.com/jackc/pgx/v5` and `golang.org/x/exp`. (PR #157)
    - Consolidated multiple project-wide dependency upgrades. (PR #114, PR #150, PR #162)

### 🗑️ Exclusions (Deprecations & Removals)
- **Relocation of Proofs:** Deleted `FORMAL_PROOF.md` from the root directory and relocated it to `simulation/FORMAL_PROOF.md` to keep verification files organized in the simulation package. (PR #145, PR #147)

### 📦 Pull Requests
- **PR #167:** fix(simulation): enforce strict error handling in chaos microservice tests.
- **PR #166:** test(logkit): refactor tests into Table-Driven Test suites and fix global state leakage.
- **PR #165:** chore: update google.golang.org/grpc to v1.82.0.
- **PR #164:** Systemic Chaos & SDET Validation (No flaws found).
- **PR #163:** test(retry): add coverage for empty option slice in DoWithValue.
- **PR #162:** build(deps): update multiple indirect dependencies.
- **PR #160:** refactor: eliminate heap allocation for retry.Config.
- **PR #159:** test(dbkit): refactor HealthCheck tests into TDT and add context cancellation edge case.
- **PR #158:** test: chaos engineering fuzz expansion and formal proof metrics.
- **PR #157:** build(deps): update pgx/v5 and x/exp dependencies.
- **PR #152:** test(async): improve coverage for QueueClient uninitialized methods.
- **PR #151:** docs: exhaustive project-wide docstring synchronization and formatting.
- **PR #147:** feat: harden concurrency and update formal proofs.
- **PR #146:** chore(deps): bump actions/checkout from 4 to 7.
- **PR #145:** test: add chaos integration test and update formal proof.
- **PR #144:** build(deps): update indirect dependencies golang.org/x/tools and golang.org/x/exp.
- **PR #141:** docs: Ensure 100% GoDoc Compliance and Fix Package Comments.
- **PR #140:** chore(deps): bump github.com/mattn/go-sqlite3 from 1.14.45 to 1.14.46 in the go-minor-patch group.
- **PR #139:** Improve edge case coverage in async package.
- **PR #138:** Update github.com/prometheus/common to v0.69.0.
- **PR #137:** docs: exhaustive documentation synchronization for gophercore.
- **PR #136:** build(deps): bump github.com/felixge/httpsnoop to v1.1.0.
- **PR #135:** docs: add escape analysis and mathematical state convergence proofs.
- **PR #133:** test(httpkit): assert slog warning on JSON write failure.
- **PR #132:** fix(testing): eliminate timer-based memory leaks in test simulations.
- **PR #131:** build(deps): bump golang.org/x/crypto and golang.org/x/net.
- **PR #130:** fix: prevent memory leaks by explicitly stopping timers in retry block.
- **PR #129:** chore(deps): bump github.com/redis/go-redis/v9 from 9.20.0 to 9.20.1 in the go-minor-patch group.
- **PR #128:** Fix goroutine leak in InMemoryCache Close.
- **PR #127:** test: improve otelkit coverage to 100% by testing initialization error paths.
- **PR #126:** merge: test empty metrics path.
- **PR #125:** merge: update sqlite3 dep consolidated.
- **PR #124:** merge: doc deep dives.
- **PR #123:** merge: apply chaos fuzz tests cleaned.
- **PR #119:** merge: resolve panic recovery test and defers.
- **PR #116:** chore(deps): update github.com/prometheus/common to v0.68.0.
- **PR #115:** docs: execute exhaustive living documentation audit across project.
- **PR #114:** build(deps): update validator and mysql dependencies.
- **PR #113:** docs: synchronize living documentation with code.
- **PR #112:** test: merge and consolidate PR #112 coverage improvements for httpkit and otelkit.
- **PR #110:** chore(deps): bump the go-minor-patch group with 7 updates.
- **PR #108:** test: improve unit test coverage in cachekit and otelkit.

---

## [v0.3.5] - OpenTelemetry, Distributed Cache, Background Jobs, and Active Context Severing

This release introduces enterprise-readiness integrations including global OpenTelemetry distributed tracing and Prometheus metrics, a new unified caching package (`cachekit`) with Redis and in-memory backends, and an `asynq` background job queue with structured panic recovery. Additionally, it hardens resilience with proactive context-severing checks across all HTTP/gRPC middleware, refactors chaos simulations to Table-Driven Test (TDT) patterns, and applies exhaustive internal logic documentation deep-dives across all core packages.

### 🚀 Additions (Features & Enhancements)
- **OpenTelemetry & Metrics Integration:** Introduced the new `otelkit` package to bootstrap OpenTelemetry SDK globally with OTLP gRPC tracer export and Prometheus metrics export. Instrumented the `httpkit` chi router using `otelchi` (exposing a `/metrics` endpoint via `promhttp`) and `grpckit` server/client handles using `otelgrpc` StatsHandlers. (PR #104)
- **Unified Cache Layer (`cachekit`):** Added a new package `cachekit` providing a unified cache interface with two implementations: a thread-safe `InMemoryCache` featuring active background TTL expiration cleanup, and a Redis-backed `RedisCache` utilizing `go-redis/v9`. (PR #106)
- **Persistent Background Jobs:** Added `asynq` integration to the `async` package, providing a Redis-backed persistent task queue. It wraps worker registration with panic recovery middleware that converts unhandled handler panics into structured `PanicError`s to prevent worker crashes. (PR #107)
- **Custom Rate Limiting Abstraction:** Decoupled `RateLimitMiddleware` from the standard `x/time/rate` package by abstracting it behind a new `RateLimiter` interface. Added `WithCustomRateLimiter` to the `httpkit` router to support custom distributed rate limiters. (PR #105)

### 🛠 Changes (Modifications & Optimizations)
- **Active Context Severing:** Hardened HTTP middlewares (`RateLimitMiddleware`, `SecurityHeadersMiddleware`, `CORSMiddleware`) and gRPC interceptors (`RecoveryUnaryInterceptor`, `RecoveryStreamInterceptor`) to check for early context expiration (`ctx.Err() != nil`). HTTP middlewares fast-path requests with a `499 Client Closed Request` status to sever dead connections and prevent resource starvation. Updated [FORMAL_PROOF.md](file:///media/zorin/HD1/projetos/GopherCore/FORMAL_PROOF.md) to define these mechanisms. (PR #99)
- **Table-Driven Chaos Testing & Security Vectors:** Refactored the microservice chaos simulation in [chaos_test.go](file:///media/zorin/HD1/projetos/GopherCore/simulation/chaos_test.go) into a Table-Driven Test (TDT) structure. Added test assertions for nil configuration, malformed payloads, SQL injection payloads, and concurrent load. Hardened `dbkit` fuzz testing to explicitly bypass SQLite driver connection fuzzing, avoiding garbage files in the workspace. (PR #99, PR #101)
- **Context Cancellation Test Rigor:** Implemented authentic table-driven tests verifying context cancellation handling for both `httpkit` middlewares and `grpckit` interceptors to achieve high statement coverage without metrics manipulation. (PR #100)
- **Internal Logic Documentation Deep-Dives:** Performed an exhaustive living documentation audit, adding detailed `// Internal Logic Deep-Dive:` docstring comments explaining complex mutex-locking, connection pooling, slice pre-allocation, and context-severing decisions across multiple packages (`async`, `circuitbreaker`, `dbkit`, `grpckit`, `guard`, `httpkit`, `retry`, `config`, `logkit`, `result`). (PR #102, PR #103)
- **Documentation Cleanup:** Cleaned up redundant `// Package` blocks in `httpkit/response.go` and `dbkit/migration.go` to conform with project standards. (PR #103)
- **Fuzzing Targets:** Added the new `cachekit` package to the Makefile fuzz targets. (PR #106)

### 📦 Pull Requests
- **PR #107:** feat(async): add asynq integration for persistent background jobs.
- **PR #106:** feat: add cachekit module with Redis and in-memory cache implementations.
- **PR #105:** feat(httpkit): abstract rate limit to allow distributed storage.
- **PR #104:** feat: integrate OpenTelemetry distributed tracing and prometheus metrics.
- **PR #103:** docs: Sync and improve package documentation.
- **PR #102:** docs: add internal logic deep-dives to core functions.
- **PR #101:** test: Refactor chaos test into table-driven pattern with edge cases.
- **PR #100:** test: improve context cancellation tests coverage.
- **PR #99:** Harden HTTP and gRPC middleware with active context severing.

---

## [v0.3.4] - Concurrency Safeguards, Security Chaos Testing, and Table-Driven Test Rigor

This release introduces critical concurrency safeguards to prevent Goroutine leaks, expands chaos simulation testing with security fuzzing profiles (SQL injection and malicious payload rejection), enforces Table-Driven Testing (TDT) patterns across all test suites to meet memory and architectural constraints, and executes exhaustive project-wide living documentation synchronizations.

### 🚀 Additions (Features & Enhancements)
- **Security Chaos Testing:** Expanded the concurrency chaos simulation in [chaos_test.go](file:///media/zorin/HD1/projetos/GopherCore/simulation/chaos_test.go) to include security fuzzing. Implemented a temporary SQLite database using `dbkit` and parameterized queries to test SQL injection resistance (`'; DROP TABLE users; --`). Introduced deeply-nested malicious JSON payloads to ensure `jsonutil` gracefully rejects malformed inputs without panicking. (PR #93)
- **100% Real Test Coverage:** Achieved 100% real test coverage on the `async` and `httpkit` packages by adding unit tests for custom context cancellation and JSON write errors. (Commit c33b463)
- **Package GoDoc:** Added package-level documentation via `doc.go` for the `grpckit` package, complying with conventions. (PR #86)

### 🛠 Changes (Modifications & Optimizations)
- **Concurrency Safeguards:** Fixed a critical goroutine leak in `async.Map` by refactoring the early-exit loop to wait for active workers, and resolved semaphore deadlocks. (PR #91)
- **Error Propagation & Logging:** Refactored `httpkit/response.go` to log write errors using `slog` instead of silently suppressing them. (PR #91)
- **Table-Driven Test (TDT) Rigor:**
    - Refactored `circuitbreaker` and `result` packages to use Table-Driven Tests (TDT) patterns for cleaner, more maintainable coverage. (PR #88)
    - Refactored `logkit` and `config` package tests to use TDT, complying with internal memory rules and architectural guidelines. (PR #95)
    - Expanded branch and edge-case unit test coverage using TDT patterns across core packages. (PR #96)
- **Living Documentation Audit:** Added missing GoDoc tags (`Purpose`, `Constraints`, `Thread-safety`) to struct fields across multiple packages (`async`, `circuitbreaker`, `dbkit`, `guard`, `httpkit`, `logkit`, `retry`). Fixed duplicate comments and incorrect package-level strings. (PR #87, PR #92, PR #98)
- **Resilience & QA Fixes:**
    - Handled context deadline exceeded in fuzz tests under high CPU contention to stabilize the CI pipeline. (Commit 10092b6)
    - Fixed unhandled Close errors in chaos tests. (PR #97)
    - Fixed deprecated `syft` subcommand usage and name/version warnings in the `Makefile`. (Commit 89a1643)
- **Dependency Upgrades:**
    - Updated standard library dependency `golang.org/x/crypto` to `v0.52.0` to resolve vulnerabilities. (Commit d079010)
    - Updated `github.com/jackc/pgx/v5` from `v5.5.4` to `v5.9.2` and `github.com/jackc/pgx/v4` from `v4.18.2` to `v4.18.3`. (PR #94)
    - Upgraded `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` to the latest version. (PR #90)

### 📦 Pull Requests
- **PR #98:** docs: exhaustive project-wide living documentation audit.
- **PR #97:** fix: catch unhandled Close errors in chaos test.
- **PR #96:** test: Improve branch and edge-case unit test coverage using TDT.
- **PR #95:** Fix architectural constraint violations in logkit and config tests.
- **PR #94:** build(deps): update jackc/pgx/v5 and jackc/pgx/v4 dependencies.
- **PR #93:** test: enhance chaos tests with SQL injection and malicious JSON payloads.
- **PR #92:** docs: exhaustive documentation synchronization.
- **PR #91:** Fix goroutine leak in async.Map and handle ignored Write error in httpkit.
- **PR #90:** chore: update go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc to latest.
- **PR #88:** test: implement Table-Driven Tests for circuitbreaker and result packages.
- **PR #87:** docs: perform exhaustive project-wide documentation update.
- **PR #86:** docs: fix grpckit missing package godoc.

---

## [v0.3.3] - Chaos Simulation, Finite State Machine Rigor, and Living Documentation Assurance

This release introduces comprehensive concurrency chaos simulation capabilities, enforces strict mathematical validation of concurrent synchronization models, refactors core test suites to robust Table-Driven Patterns (TDT) for `circuitbreaker` and `result` packages, resolves a critical half-open request leak in Circuit Breaker state transitions, and achieves 100% compliance with the repository's strict "Living Documentation" philosophy.

### 🚀 Additions (Features & Enhancements)
- **Chaos & Resilience Testing (Simulation):** Introduced [chaos_test.go] to validate GopherCore components (HTTP, JSON, Retry, Circuit Breaker) under concurrent stress and randomized latency, ensuring no data races or goroutine leaks under extreme conditions. (Commit f3d7023)
- **Formal Verification (Documentation):** Added [FORMAL_PROOF.md] describing mathematical formulations, invariant rules, and O(1) performance characteristics behind the toolkit's synchronization strategy. (Commit f3d7023)

### 🛠 Changes (Modifications & Optimizations)
- **Resilience & State Integrity (CircuitBreaker):**
    - Refactored `Execute` in [circuitbreaker.go] to use defer blocks for state mutations, fixing a half-open request leak where `halfOpenRequests` would not decrement properly if user functions failed, leaving the circuit stuck returning `ErrTooManyRequests`. (PR #82)
    - Hardened panic recovery paths inside concurrent user execution loops to preserve mutex state consistency before bubble-up. (PR #82)
- **QA & Test Rigor:**
    - Refactored all standalone tests in the `async` and `guard` packages into high-coverage, parameter-driven Table-Driven Test (TDT) structures. (PR #78)
    - Expanded TDT coverage in `circuitbreaker` and `result` packages to achieve an absolute **100.0% statement coverage** across all modules, including full validation of panic propagation paths. (PR #84)
- **Living Documentation Audit:** Conducted a comprehensive repository-wide Abstract Syntax Tree (AST) validation to ensure every exported function, type, and package symbol maintains complete, non-superficial inline docs (Purpose, Constraints, and Thread-Safety tags). (PR #83)
- **CI/CD Maintenance:** Upgraded the SLSA Level 3 dynamic provenance generator configuration in release pipelines to `v2.1.0`. (PR #79)

### 📦 Pull Requests
- **PR #85:** test: implement chaos simulations and formal mathematical proofs.
- **PR #84:** test(core): improve test rigor with Table-Driven Tests for circuitbreaker and result packages.
- **PR #83:** docs: maintain living documentation and eliminate superficial comments.
- **PR #82:** fix(circuitbreaker): resolve half-open request leak and panic handling.
- **PR #79:** chore(deps): bump slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml from 2.0.0 to 2.1.0 in the actions-minor-patch group.
- **PR #78:** test: refactor async and guard tests to comprehensive TDT structures.

---

## [v0.3.2] - QA Resilience, Nil-Safety, and Windows Test Compatibility

This release focuses on strengthening the project's reliability by addressing potential nil-pointer panics in the `retry` package, improving test coverage and error handling in `dbkit` migrations, and ensuring full test suite compatibility with Windows environments.

### 🚀 Additions (Features & Enhancements)
- **QA & Test Resilience (dbkit):** Introduced `migration_close_test.go` with comprehensive table-driven tests for deferred close errors in database migrations. (Commit 5dd07a6)

### 🛠 Changes (Modifications & Optimizations)
- **Reliability (Retry):** Fixed a potential nil-pointer panic in `Do` and `DoWithValue` loops by adding explicit checks for nil options. (PR #71)
- **Security & QA (dbkit):**
    - Fixed a `gosec G104` violation by ensuring `m.Close()` errors are handled gracefully via named returns in migrations. (PR #72)
    - Improved fuzz testing robustness by properly handling `db.Close()` errors. (PR #70)
- **QA & Test Coverage:** Optimized table-driven tests for `dbkit` options and expanded coverage across core utilities. (PR #69, #75)
- **Compatibility:** Fixed Windows-specific path formatting and handling issues in migration tests to ensure cross-platform CI stability. (Commits a83df74, 8a75cb4, 97bf33d)
- **Maintenance:** Updated all project dependencies to their latest stable versions, including critical updates to `google.golang.org/grpc`. (PR #76)

### 📦 Pull Requests
- **PR #76:** chore(deps): bump google.golang.org/grpc in the go-minor-patch group.
- **PR #75:** fix: improve dbkit test coverage and resilience.
- **PR #72:** fix: address gosec G104 in migration closing logic.
- **PR #71:** fix: prevent nil-pointer panic in retry options.
- **PR #70:** fix(dbkit): handle database close errors in fuzz tests.
- **PR #69:** test: implement comprehensive table-driven tests for dbkit options.

---

## [v0.3.1] - CI/CD Maintenance and SLSA Provenance Fix

This patch release fixes the automated SBOM generation in the release pipeline and enables SLSA Level 3 provenance asset uploading to GitHub Releases.

### 🛠 Changes (Modifications & Optimizations)
- **CI/CD:** Fixed SBOM generation parameter in `release.yml` and enabled `upload-assets` for SLSA provenance.

---

## [v0.3.0] - gRPC Integration, Fuzz Testing, and DevSecOps Hardening

This major minor release introduces a production-ready gRPC package (`grpckit`), implements exhaustive fuzz testing across the entire toolkit, and significantly strengthens the project's security posture with CodeQL and SLSA Level 3 provenance integration.

### 🚀 Additions (Features & Enhancements)
- **gRPC Toolkit (`grpckit`):** Introduced a comprehensive package for building resilient gRPC services, including production-ready server and client implementations, middleware/interceptors, and 100% test coverage. (Commit ca6b98b, f57258b)
- **DevSecOps (Security):**
    - **CodeQL Integration:** Integrated GitHub CodeQL for automated Static Analysis Security Testing (SAST) and advanced taint tracking to detect potential vulnerabilities. (PR #67)
    - **SLSA & SBOM:** Implemented SLSA (Supply-chain Levels for Software Artifacts) Level 3 provenance and automated Software Bill of Materials (SBOM) generation in SPDX format (`gophercore-sbom.spdx.json`). (PR #68)
- **QA & Testing:**
    - **Exhaustive Fuzz Testing:** Implemented project-wide fuzz testing for core packages (`async`, `config`, `dbkit`, `grpckit`, `httpkit`, `logkit`) to identify edge-case bugs and ensure robustness against malformed input. (PR #62)

### 🛠 Changes (Modifications & Optimizations)
- **QA Resilience:** Fixed flakiness in the `retry` package's context cancellation tests by disabling jitter during specific test scenarios. (Commit cfc88a2)
- **Documentation (Technical Writing):**
    - Performed a repository-wide "Living Documentation" audit to ensure absolute synchronization between code and comments. (PR #66)
    - Added detailed internal logic comments explaining complex architectural decisions and corrected technical inaccuracies. (PR #60)
- **Maintenance:** Performed a global update of all dependencies to their latest stable versions and optimized the module graph. (PR #61)

### 📦 Pull Requests
- **PR #68:** ci: integrate SBOM and SLSA Level 3 provenance.
- **PR #67:** ci: integrate CodeQL for SAST and taint tracking.
- **PR #66:** docs: exhaustive living documentation audit across repository.
- **PR #62:** test: implement exhaustive fuzz testing across core packages.
- **PR #61:** build(deps): update project dependencies to latest versions.
- **PR #60:** docs: add internal logic comments and correct inaccuracies.

---

## [v0.2.3] - Security Hardening, Test Concurrency, and Godoc Maturity

This release strengthens the project's security posture by updating the Go runtime and focuses on the "Living Documentation" philosophy by adding inline reasoning to complex logic. It also significantly improves QA resilience with new concurrency tests and a transition to Table-Driven Testing (TDT) for core packages.

### 🚀 Additions (Features & Enhancements)
- **QA & Concurrency (HTTP):** Implemented high-concurrency stress tests for the `httpkit` response package to ensure thread-safety under heavy load. (PR #57, #59)
- **Security (Go Runtime):** Updated the project's Go version to `1.26.3` to incorporate critical security patches and vulnerability fixes in the standard library. (PR #59)

### 🛠 Changes (Modifications & Optimizations)
- **QA Resilience (TDT):**
    - Refactored `httpkit` middleware and response test suites to use strict Table-Driven Testing (TDT), improving maintainability and edge-case coverage. (PR #59)
    - Transitioned the `result` package test suite to the TDT pattern. (PR #54)
- **Documentation (Technical Writing):**
    - Exhaustive rewrite of all exported `godoc` strings across the repository, focusing on clarity, thread-safety guarantees, and usage examples. (PR #55)
    - Added inline documentation providing "reasoning" for architectural decisions within the source code. (PR #55)
- **CI/CD Optimization:** Added a 1MB payload limit to string fuzzing in the `guard` package to prevent Out-Of-Memory (OOM) errors and stabilize CI pipeline execution time. (PR #56)
- **Maintenance:** Updated internal dependencies and performed a global `go mod tidy` audit. (PR #59)

### 📦 Pull Requests
- **PR #59:** test: Enhance httpkit test coverage with TDT pattern (Includes go.mod update and concurrency tests).
- **PR #57:** test(httpkit): refactor response tests to TDT and validate thread-safety.
- **PR #56:** docs: audit and verify Living Documentation (Fuzz size limit).
- **PR #55:** docs: exhaustive rewrite of exported godoc strings and inline reasoning.
- **PR #54:** test: refactor result tests to use strict table-driven testing.

---

## [v0.2.2] - Documentation Audit and Consistency

This patch release focuses strictly on a comprehensive audit of the project's living documentation to ensure absolute consistency between implementation and documentation.

### 🛠 Changes (Modifications & Optimizations)
- **Documentation:** Performed a full-repository audit and synchronization of `godoc` strings to maintain the highest standard of technical writing. (PR #51)

### 📦 Pull Requests
- **PR #51:** docs: complete full-repository living documentation audit.

---

## [v0.2.1] - Maintenance, Test Coverage, and Security Refinement

This patch release focuses on increasing test coverage, refining security configurations, and maintaining the project's living documentation. It addresses a CORS vulnerability, optimizes performance in HTTP middlewares, and ensures full compatibility with Go 1.26.

### 🚀 Additions (Features & Enhancements)
- **Security (HTTP):** Added `IdleTimeout` configuration to the HTTP server, strengthening protection against resource exhaustion. (PR #36)
- **CI/CD Pipeline:** Refactored CI/release workflows to use dynamic Go versioning and centralized Make targets, improving build reproducibility. (PR #42)

### 🛠 Changes (Modifications & Optimizations)
- **Security (CORS):** Fixed a vulnerability in the CORS middleware that could allow unauthorized origins in specific edge cases. (PR #46)
- **Performance:** Optimized `Header` allocation in the HTTP middleware stack to reduce memory overhead and GC pressure. (PR #46)
- **QA & Test Resilience:**
    - Significantly increased branch coverage for `retry`, `circuitbreaker`, `dbkit`, `guard`, and `httpkit` packages. (PR #50, #49, #32)
    - Implemented Table-Driven concurrency tests for the `result` and `retry` packages to ensure thread-safety under heavy load. (PR #45)
    - Improved unit test coverage for edge cases across the entire toolkit. (PR #41)
    - Fixed flaky context cancellation tests in the `retry` package.
- **Documentation:** Performed an exhaustive, project-wide `godoc` synchronization and audit to maintain technical writing standards. (PR #43, #38, #48, #47)
- **Maintenance:**
    - Resolved CI lint issues caused by the deprecation of `reflect.Ptr` in Go 1.26. (PR #46)
    - Updated internal and external dependencies to their latest stable versions. (PR #44, #40)

### 📦 Pull Requests
- **PR #50:** Improve branch coverage for `retry`, `circuitbreaker`, and `dbkit`.
- **PR #49:** Increase test coverage for the `guard` package.
- **PR #48:** Technical Writing audit for documentation consistency.
- **PR #47:** Fix documentation synchronization in `httpkit`.
- **PR #46:** Fix CORS vulnerability and optimize HTTP middleware allocations.
- **PR #45:** Add Table-Driven concurrency tests to `result` and `retry`.
- **PR #44:** Update project-wide dependencies.
- **PR #43:** Exhaustive project-wide docstring synchronization.
- **PR #42:** Refactor CI workflows for dynamic Go versioning.
- **PR #41:** Improve unit test coverage for edge cases.
- **PR #40:** Bump `github.com/mattn/go-sqlite3` dependency.
- **PR #38:** Exhaustive project-wide godoc synchronization.
- **PR #37:** Improve `retry` package coverage.
- **PR #36:** Add `IdleTimeout` to HTTP server and security hardening.
- **PR #32:** Expand `httpkit` branch coverage and concurrency tests.

---

## [v0.2.0] - Security, QA Resilience, and Documentation Maturity

This release consolidates significant structural work on the fundamental base of the `GopherCore` repository. The focus was directed towards three major pillars: **DevSecOps Security, Test Resilience (QA), and Documentation Maturity**.

### 🚀 Additions (Features & Enhancements)
- **Security (DevSecOps):** Implemented native mitigation against **Slowloris** attacks and fixed **Integer Overflow** vulnerabilities.
- **Security (Crypto):** Replaced the weak random number generator (`math/rand`) with the robust `crypto/rand` for `jitter` calculation in the Retry package, strengthening network call security.
- **HTTP Configurations:** Added explicit support for the `ReadHeaderTimeout` configuration in HTTP servers, promoting a secure-by-default standard.
- **Quality Tooling:** Integrated strict new linters into the CI/CD pipeline (`nilnil`, `govet nilness`, `NilAway`), eliminating entire classes of bugs involving nil-pointers.
- **Test Coverage (QA):** Added robust tests for server `graceful shutdown` and completely refactored the HTTP test suites using *Table-Driven Tests (TDT)* with mass concurrency guarantees.

### 🛠 Changes (Modifications & Optimizations)
- **HTTP Performance:** Optimized slice pre-allocation in validation functions (`guard`) and continuous optimizations in HTTP middlewares to reduce Garbage Collection (GC) pressure.
- **Documentation (Technical Writing):** Conducted a repository-wide audit, resulting in high-level `godoc` synchronization regarding *Thread-safety*, function purity, and *Constraints* across the `retry`, `result`, `config`, `dbkit`, and `httpkit` packages.
- **Pipeline and Build:** Adjusted the `Makefile` to isolate tools in `GOBIN_PATH` and performed critical updates on external standard module dependencies.
- **CI Stabilization:** Locked `execution count` usage in fuzzing tests, mitigating flakiness caused by context deadlines in concurrent GitHub Actions environments.

### 🗑️ Exclusões (Deprecations & Removals)
- **Dead/Obsolete Code Removal:** Completely removed the `bench_test.go` file as it relied on legacy header mappings (`rr.HeaderMap`) deprecated in recent standard library versions, which was causing noise in the test suite.

### 📦 Pull Requests
The following Pull Requests were merged into the main branch for this release:
- **PR #29:** Improve edge case and concurrency coverage in `httpkit`.
- **PR #27:** Exhaustive documentation audit for thread-safety and constraints.
- **PR #26:** Optimize HTTP header allocations.
- **PR #25:** Fix Security vulnerabilities (Slowloris and Integer overflow).
- **PR #24:** Exhaustive project-wide living documentation sync.
- **PR #23:** Add `nilnil` and `govet` nilness checks to lint.
- **PR #22:** Fix `golangci` config and make lint pass with NilAway.
- **PR #21:** Add `ReadHeaderTimeout` to `http.Server` config.
- **PR #20:** Add test for graceful shutdown server close.
- **PR #18:** Update standard module dependencies.
- **PR #17:** Exhaustive project-wide living documentation sync.
- **PR #16:** Increase config unit test coverage to 100%.
- **PR #15:** Optimize HTTP middleware allocations.
- **PR #14:** Replace weak random number generator with `crypto/rand` for retry jitter.
- **PR #13:** Optimize GC by pre-allocating `errs slice` in guard.
- **PR #12:** Bump Github Actions base releases.

*(QA Note: Duplicate PRs #28 and #30 were identified, blocked due to native canonicalization bypass risks, and closed without merging).*

---

## [v0.1.0] - Initial Release and Foundation

The first official release of the GopherCore modular toolkit, laying the foundations for resilient Go development.

### 🚀 Additions
- **Core Packages:** Released the complete initial `GopherCore` modular Go toolkit.
- **Configuration Management:** Added the `configkit` package featuring reflection safety for robust environment parsing.
- **Logging:** Added the structured logging package `logkit`.
- **HTTP Tooling:** Introduced the `GracefulShutdown` utility in `httpkit`.

### 🛠 Changes
- **Security & Parsing:** Refactored `StripHTML` to utilize the robust `microcosm-cc/bluemonday` engine.
- **Runtime:** Updated the project's Go version in `go.mod` to `1.26.0` to utilize the latest compiler improvements.
- **Refactoring:** Extracted duplicate router configuration logic to adhere to DRY principles.
- **CI/CD:** Resolved multiple CI pipeline issues, fixing Gosec SARIF missing errors and Lint binary mismatches by installing tools from source via `go install`.

### 🗑️ Exclusions
- **Cleanup:** Removed runtime logs from the git hierarchy.

### 📦 Pull Requests
- **PR #11:** Refactor duplicate router configuration logic.
- **PR #9:** Add structured logging package `logkit`.
- **PR #8:** Add `GracefulShutdown` utility in `httpkit`.
- **PR #7:** Refactor `StripHTML` to use `microcosm-cc/bluemonday`.
- **PR #6:** Update `go.mod` version to `1.26.0`.
