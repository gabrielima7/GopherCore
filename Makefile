.PHONY: all lint nilaway test fuzz security vulncheck audit tidy fmt vet install-tools help

# Default target
all: audit

## help: Show this help message
help:
	@echo "GopherCore — Makefile targets:"
	@echo ""
	@echo "  make lint        Run golangci-lint + NilAway"
	@echo "  make nilaway     Run NilAway static nil analysis"
	@echo "  make test        Run tests with coverage and race detector"
	@echo "  make fuzz        Run fuzz tests (30s per target)"
	@echo "  make security    Run gosec security analysis"
	@echo "  make vulncheck   Run govulncheck dependency audit"
	@echo "  make audit       Run all checks (lint + test + security + vulncheck)"
	@echo "  make tidy        Tidy go.mod and go.sum"
	@echo "  make fmt         Format all Go files"
	@echo "  make vet         Run go vet"
	@echo "  make install-tools  Install required development tools"
	@echo ""

## install-tools: Install golangci-lint, nilaway, gosec, govulncheck
install-tools:
	@echo "==> Installing development tools..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install go.uber.org/nilaway/cmd/nilaway@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "==> Done."

## fmt: Format all Go source files
fmt:
	@echo "==> Formatting..."
	gofmt -s -w .

## vet: Run go vet
vet:
	@echo "==> Running go vet..."
	go vet ./...

## nilaway: Run NilAway static nil dereference analysis
nilaway:
	@echo "==> Running NilAway..."
	go install go.uber.org/nilaway/cmd/nilaway@latest
	nilaway ./...

## lint: Run golangci-lint + NilAway
lint:
	@echo "==> Running linters..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@GOBIN_PATH="$$(go env GOBIN)"; \
	if [ -z "$$GOBIN_PATH" ]; then \
		GOBIN_PATH="$$(go env GOPATH)/bin"; \
	fi; \
	"$$GOBIN_PATH/golangci-lint" run ./...
	$(MAKE) nilaway

## test: Run tests with coverage and race detector
test:
	@echo "==> Running tests..."
	go test -cover -race -count=1 -timeout=60s ./...

## fuzz: Run fuzz tests for 30s per package
fuzz:
	@echo "==> Running fuzz tests..."
	@for pkg in result retry circuitbreaker guard jsonutil; do \
		echo "  -> Fuzzing $$pkg..."; \
		go test -fuzz=. -fuzztime=30s ./$$pkg/ || exit 1; \
	done

## security: Run gosec static security analysis
security:
	@echo "==> Running gosec..."
	gosec -quiet ./...

## vulncheck: Run govulncheck dependency vulnerability check
vulncheck:
	@echo "==> Running govulncheck..."
	govulncheck ./...

## tidy: Clean up go.mod and go.sum
tidy:
	@echo "==> Tidying modules..."
	go mod tidy

## audit: Run all checks (lint + test + security + vulncheck)
audit: lint vet test security vulncheck
	@echo ""
	@echo "==> All checks passed!"
