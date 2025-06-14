# Makefile for building pulse examples for macOS, Linux and Windows
.PHONY: all clean mac linux windows examples mac-race linux-race windows-race test test-coverage ci-test lint lint-fix

# Default target
all: examples

# Test and CI targets
test:
	@echo "Running tests..."
	go test -v -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic . ./core/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

ci-test:
	@echo "Running CI tests..."
	go test -race -coverprofile=coverage.out -covermode=atomic . ./core/...

lint:
	@echo "Running linter..."
	golangci-lint run --timeout=5m . core task/...

lint-fix:
	@echo "Running linter with auto-fix..."
	golangci-lint run --timeout=5m . core task/... --fix

# Directories
EXAMPLE_DIR = example
BIN_DIR = bin
MAC_DIR = $(BIN_DIR)/mac
LINUX_DIR = $(BIN_DIR)/linux
WINDOWS_DIR = $(BIN_DIR)/windows

# Go command and flags
GO = go
GO_BUILD = $(GO) build
GO_FLAGS = -v
GO_RACE_FLAGS = -v -race
GOOS_MAC = darwin
GOOS_LINUX = linux
GOOS_WINDOWS = windows

# Examples to build
EXAMPLES = client server

# Create necessary directories
$(MAC_DIR):
	mkdir -p $(MAC_DIR)

$(LINUX_DIR):
	mkdir -p $(LINUX_DIR)

$(WINDOWS_DIR):
	mkdir -p $(WINDOWS_DIR)

# Build all examples
examples: mac linux windows

# Build macOS examples
mac: $(MAC_DIR)
	@echo "Building macOS examples..."
	GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_FLAGS) -o $(MAC_DIR)/client $(EXAMPLE_DIR)/core/client/client.go
	GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_FLAGS) -o $(MAC_DIR)/server $(EXAMPLE_DIR)/core/server/server.go
	GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_FLAGS) -o $(MAC_DIR)/echo_client $(EXAMPLE_DIR)/echo/client/client.go
	GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_FLAGS) -o $(MAC_DIR)/echo_server $(EXAMPLE_DIR)/echo/server/server.go
	@echo "macOS examples built successfully in $(MAC_DIR)/"

# Build macOS examples with race detection
mac-race: $(MAC_DIR)
	@echo "Building macOS examples with race detection..."
	CGO_ENABLED=1 GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(MAC_DIR)/client_race $(EXAMPLE_DIR)/core/client/client.go
	CGO_ENABLED=1 GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(MAC_DIR)/server_race $(EXAMPLE_DIR)/core/server/server.go
	CGO_ENABLED=1 GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(MAC_DIR)/echo_client_race $(EXAMPLE_DIR)/echo/client/client.go
	CGO_ENABLED=1 GOOS=$(GOOS_MAC) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(MAC_DIR)/echo_server_race $(EXAMPLE_DIR)/echo/server/server.go
	@echo "macOS examples with race detection built successfully in $(MAC_DIR)/"

# Build Linux examples
linux: $(LINUX_DIR)
	@echo "Building Linux examples..."
	GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_FLAGS) -o $(LINUX_DIR)/client $(EXAMPLE_DIR)/core/client/client.go
	GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_FLAGS) -o $(LINUX_DIR)/server $(EXAMPLE_DIR)/core/server/server.go
	GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_FLAGS) -o $(LINUX_DIR)/echo_client $(EXAMPLE_DIR)/echo/client/client.go
	GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_FLAGS) -o $(LINUX_DIR)/echo_server $(EXAMPLE_DIR)/echo/server/server.go
	@echo "Linux examples built successfully in $(LINUX_DIR)/"

# Build Linux examples with race detection
linux-race: $(LINUX_DIR)
	@echo "Building Linux examples with race detection..."
	CGO_ENABLED=1 GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(LINUX_DIR)/client_race $(EXAMPLE_DIR)/core/client/client.go
	CGO_ENABLED=1 GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(LINUX_DIR)/server_race $(EXAMPLE_DIR)/core/server/server.go
	CGO_ENABLED=1 GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(LINUX_DIR)/echo_client_race $(EXAMPLE_DIR)/echo/client/client.go
	CGO_ENABLED=1 GOOS=$(GOOS_LINUX) $(GO_BUILD) $(GO_RACE_FLAGS) -o $(LINUX_DIR)/echo_server_race $(EXAMPLE_DIR)/echo/server/server.go
	@echo "Linux examples with race detection built successfully in $(LINUX_DIR)/"

# Build Windows examples
windows: $(WINDOWS_DIR)
	@echo "Building Windows examples..."
	GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_FLAGS) -o $(WINDOWS_DIR)/client.exe $(EXAMPLE_DIR)/core/client/client.go
	GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_FLAGS) -o $(WINDOWS_DIR)/server.exe $(EXAMPLE_DIR)/core/server/server.go
	GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_FLAGS) -o $(WINDOWS_DIR)/echo_client.exe $(EXAMPLE_DIR)/echo/client/client.go
	GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_FLAGS) -o $(WINDOWS_DIR)/echo_server.exe $(EXAMPLE_DIR)/echo/server/server.go
	@echo "Windows examples built successfully in $(WINDOWS_DIR)/"

# Build Windows examples with race detection
windows-race: $(WINDOWS_DIR)
	@echo "Building Windows examples with race detection..."
	CGO_ENABLED=1 GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_RACE_FLAGS) -o $(WINDOWS_DIR)/client_race.exe $(EXAMPLE_DIR)/core/client/client.go
	CGO_ENABLED=1 GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_RACE_FLAGS) -o $(WINDOWS_DIR)/server_race.exe $(EXAMPLE_DIR)/core/server/server.go
	CGO_ENABLED=1 GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_RACE_FLAGS) -o $(WINDOWS_DIR)/echo_client_race.exe $(EXAMPLE_DIR)/echo/client/client.go
	CGO_ENABLED=1 GOOS=$(GOOS_WINDOWS) GOARCH=amd64 $(GO_BUILD) $(GO_RACE_FLAGS) -o $(WINDOWS_DIR)/echo_server_race.exe $(EXAMPLE_DIR)/echo/server/server.go
	@echo "Windows examples with race detection built successfully in $(WINDOWS_DIR)/"

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)
	@echo "Cleaned build artifacts"
