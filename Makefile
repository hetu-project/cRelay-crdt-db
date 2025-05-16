.PHONY: all build test clean install deps

# Default target
all: deps build

build:
	go build -o bin/cRelay-crdt-db ./cmd/main.go

# Install to GOPATH/bin
install: deps
	go build -o $(GOPATH)/bin/cRelay-crdt-db ./cmd/main.go

# Run all tests
test: deps
	go test -v ./...

# Run tests for specific package
test-handlers: deps
	go test -v ./internal/api/handlers/...

# Run specific test file
test-event-handlers: deps
	go test -v ./internal/api/handlers/event_handlers_test.go

# Run orbitdb package tests
test-orbitdb: deps
	go test -v ./orbitdb/...

# Run specific orbitdb test file
test-orbitdb-adapter: deps
	go test -v ./orbitdb/adapter_test.go

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Install and update dependencies
deps:
	go mod download
	go mod tidy 