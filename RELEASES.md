# GopherCore Releases

This document tracks all major additions, alterations, deletions, and pull requests merged for each version of the GopherCore project.

---

## [v0.2.0] - Security, QA Resilience, and Documentation Maturity

This release consolidates significant structural work on the fundamental base of the `GopherCore` repository. The focus was directed towards three major pillars: **DevSecOps Security, Test Resilience (QA), and Documentation Maturity**.

### 🚀 Additions (Features & Enhancements)
- **Security (DevSecOps):** Implemented native mitigation against **Slowloris** attacks and fixed **Integer Overflow** vulnerabilities.
- **Security (Crypto):** Replaced the weak random number generator (`math/rand`) with the robust `crypto/rand` for `jitter` calculation in the Retry package, strengthening network call security.
- **HTTP Configurations:** Added explicit support for the `ReadHeaderTimeout` configuration in HTTP servers, promoting a secure-by-default standard.
- **Quality Tooling:** Integrated strict new linters into the CI/CD pipeline (`nilnil`, `govet nilness`, `NilAway`), eliminating entire classes of bugs involving nil-pointers.
- **Test Coverage (QA):** Added robust tests for server `graceful shutdown` and completely refactored the HTTP test suites using *Table-Driven Tests (TDT)* with mass concurrency guarantees.

### 🛠 Alterações (Modifications & Optimizations)
- **HTTP Performance:** Optimized slice pre-allocation in validation functions (`guard`) and continuous optimizations in HTTP middlewares to reduce Garbage Collection (GC) pressure.
- **Exhaustive Documentation (Technical Writing):** Conducted a repository-wide audit, resulting in high-level `godoc` synchronization regarding *Thread-safety*, function purity, and *Constraints* across the `retry`, `result`, `config`, `dbkit`, and `httpkit` packages.
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

### 🛠 Alterações
- **Security & Parsing:** Refactored `StripHTML` to utilize the robust `microcosm-cc/bluemonday` engine.
- **Runtime:** Updated the project's Go version in `go.mod` to `1.26.0` to utilize the latest compiler improvements.
- **Refactoring:** Extracted duplicate router configuration logic to adhere to DRY principles.
- **CI/CD:** Resolved multiple CI pipeline issues, fixing Gosec SARIF missing errors and Lint binary mismatches by installing tools from source via `go install`.

### 🗑️ Exclusões
- **Cleanup:** Removed runtime logs from the git hierarchy.

### 📦 Pull Requests
- **PR #11:** Refactor duplicate router configuration logic.
- **PR #9:** Add structured logging package `logkit`.
- **PR #8:** Add `GracefulShutdown` utility in `httpkit`.
- **PR #7:** Refactor `StripHTML` to use `microcosm-cc/bluemonday`.
- **PR #6:** Update `go.mod` version to `1.26.0`.
