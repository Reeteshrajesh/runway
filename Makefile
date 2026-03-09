BINARY     := runway
MODULE     := github.com/Reeteshrajesh/runway
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags="-s -w -X main.version=$(VERSION)"
BUILD_DIR  := dist

.PHONY: build install clean lint test vet release help

## build: compile binary for the current platform
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) ./cmd/runway

## install: build and install to /usr/local/bin
install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	@echo "installed $(BINARY) to /usr/local/bin"

## test: run all tests with race detector
test:
	go test -race -count=1 ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run vet (add golangci-lint here if desired)
lint: vet

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)/

## release: build binaries for all supported platforms
release: clean
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64   ./cmd/runway
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64   ./cmd/runway
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64  ./cmd/runway
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64  ./cmd/runway
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/runway
	cd $(BUILD_DIR) && sha256sum * > SHA256SUMS
	@echo "binaries in $(BUILD_DIR)/"

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
