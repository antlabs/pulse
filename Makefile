# Makefile for building pulse examples for macOS and Linux
.PHONY: all clean mac linux examples mac-race linux-race

# Default target
all: examples

# Directories
EXAMPLE_DIR = example
BIN_DIR = bin
MAC_DIR = $(BIN_DIR)/mac
LINUX_DIR = $(BIN_DIR)/linux

# Go command and flags
GO = go
GO_BUILD = $(GO) build
GO_FLAGS = -v
GO_RACE_FLAGS = -v -race
GOOS_MAC = darwin
GOOS_LINUX = linux

# Examples to build
EXAMPLES = client server

# Create necessary directories
$(MAC_DIR):
	mkdir -p $(MAC_DIR)

$(LINUX_DIR):
	mkdir -p $(LINUX_DIR)

# Build all examples
examples: mac linux

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

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)
	@echo "Cleaned build artifacts"
