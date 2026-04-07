# Binary name
binary_name := "story-factory"

# Default recipe - show help
default:
    @just --list

# Build the application
build:
    go build -o {{binary_name}} ./cmd/story-factory

# Install the binary to $GOPATH/bin
install:
    go install ./cmd/story-factory

# Run all tests
test:
    go test ./...

# Run tests with coverage report
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report generated: coverage.html"

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Run tests for a specific package (e.g., just test-pkg ./internal/claude)
test-pkg pkg:
    go test -v {{pkg}}

# Clean build artifacts
clean:
    rm -f {{binary_name}}
    rm -f coverage.out coverage.html

# Run linter (requires golangci-lint)
lint:
    golangci-lint run ./...

# Format code
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Run fmt, vet, and test
check: fmt vet test

# Build and run with arguments (e.g., just run --help)
run *args: build
    ./{{binary_name}} {{args}}
