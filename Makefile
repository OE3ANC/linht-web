.PHONY: build build-arm build-arm64 run clean install test

# Variables
BINARY_NAME=linht-web
BUILD_DIR=./build

# Build for current platform
build:
	@echo "Building for current platform..."
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for ARM64
build-arm64:
	@echo "Building for ARM64..."
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-arm64"

# Build all platforms
build-all: build build-arm64
	@echo "All builds complete"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	go run main.go

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

# Install to system (requires sudo)
install: build
	@echo "Installing to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build for current platform"
	@echo "  build-arm64 - Build for ARM64"
	@echo "  build-all   - Build for all platforms"
	@echo "  run         - Run the application"
	@echo "  clean       - Remove build artifacts"
	@echo "  deps        - Install dependencies"
	@echo "  install     - Install to /usr/local/bin (requires sudo)"
	@echo "  help        - Show this help message"