# Makefile
.PHONY: build install clean test fmt vet

# Binary name
BINARY=upkg
VERSION=0.1.0

# Build the binary
build:
	go build -o bin/$(BINARY) cmd/upkg/main.go

# Install to system
install: build
	sudo cp bin/$(BINARY) /usr/local/bin/

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linter
lint:
	golangci-lint run

# Build for multiple platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 cmd/upkg/main.go
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 cmd/upkg/main.go
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 cmd/upkg/main.go
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY)-linux-arm64 cmd/upkg/main.go