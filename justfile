# google-discovery-mcp justfile
# Run `just` to see available targets

# Default target
default: build

# Build the binary
build:
    go build -o google-discovery-mcp .

# Build with race detector
build-race:
    go build -race -o google-discovery-mcp .

# Install the binary to $GOPATH/bin
install:
    go install .

# Run all tests
test:
    go test -v ./...

# Run tests with race detector
test-race:
    go test -race -v ./...

# Run tests with coverage
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Run linter
lint:
    golangci-lint run ./...

# Run linter and fix issues
lint-fix:
    golangci-lint run --fix ./...

# Format code
fmt:
    go fmt ./...
    gofumpt -w .

# Run go vet
vet:
    go vet ./...

# Tidy dependencies
tidy:
    go mod tidy

# Clean build artifacts
clean:
    rm -f google-discovery-mcp
    rm -f coverage.out coverage.html

# Run all quality checks (format, lint, vet, test)
quality: fmt lint vet test

# Pre-commit hook: run before committing
pre-commit: fmt lint vet test
    @echo "All checks passed!"

# CI target: run all checks without modifying files
ci: lint vet test

# Example: list all Google APIs
example-list:
    go run . -list

# Example: list YouTube methods
example-youtube-methods:
    go run . -api youtube -version v3 -list-methods

# Example: generate YouTube video tools
example-youtube-videos:
    go run . -api youtube -version v3 -methods videos.list,videos.insert,videos.update

# Example: generate Drive file tools
example-drive:
    go run . -api drive -version v3 -methods files.list,files.get

# Check for available dependency updates
check-updates:
    go list -u -m all

# Update all dependencies
update-deps:
    go get -u ./...
    go mod tidy

# Show environment info
env-info:
    @echo "Go version: $(go version)"
    @echo "GOPATH: $GOPATH"
    @echo "GOBIN: $GOBIN"
    @which golangci-lint && golangci-lint --version || echo "golangci-lint not installed"

# Install development dependencies
install-deps:
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install mvdan.cc/gofumpt@latest
