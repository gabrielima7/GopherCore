# GopherCore Releases

This document tracks all major additions, alterations, deletions, and pull requests merged for each version of the GopherCore project.

---

## [v0.3.1] - gRPC Integration, Fuzz Testing, and DevSecOps Hardening

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
