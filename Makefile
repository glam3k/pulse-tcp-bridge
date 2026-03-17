# Makefile

# Detect OS
UNAME_S := $(shell uname -s)

# Docker image for building
DOCKER_IMAGE = pulse-bridge-builder
DOCKERFILE = Dockerfile.build

# Default target
all: build

# Build the Go binary for the current architecture
# Note: This will likely fail on macOS due to CGo dependencies.
# Use build-amd64 or build-arm64 for cross-compilation.
build:
	go build -o bin/pulse-tcp-bridge .

# Build for linux/amd64
build-amd64: builder-image
ifeq ($(UNAME_S),Darwin)
	@echo "Building for linux/amd64 via Docker..."
	docker run --rm -v $(CURDIR):/app -w /app \
		-e GOOS=linux -e GOARCH=amd64 \
		-e CGO_ENABLED=1 \
		-e CC=x86_64-linux-gnu-gcc \
		$(DOCKER_IMAGE) go build -v -o bin/pulse-tcp-bridge-amd64 .
else
	@echo "Building for linux/amd64 natively..."
	GOOS=linux GOARCH=amd64 go build -v -o bin/pulse-tcp-bridge-amd64 .
endif

# Build for linux/arm64
build-arm64: builder-image
ifeq ($(UNAME_S),Darwin)
	@echo "Building for linux/arm64 via Docker..."
	docker run --rm -v $(CURDIR):/app -w /app \
		-e GOOS=linux -e GOARCH=arm64 \
		$(DOCKER_IMAGE) go build -v -o bin/pulse-tcp-bridge-arm64 .
else
	@echo "Building for linux/arm64 natively..."
	GOOS=linux GOARCH=arm64 go build -v -o bin/pulse-tcp-bridge-arm64 .
endif

# Build the Docker builder image
builder-image: $(DOCKERFILE)
	@docker image inspect $(DOCKER_IMAGE) >/dev/null 2>&1 || \
		(echo "Builder image not found, building..." && docker build -f $(DOCKERFILE) -t $(DOCKER_IMAGE) .)

# Clean the build artifacts
clean:
	rm -rf bin

.PHONY: all build build-amd64 build-arm64 builder-image clean
