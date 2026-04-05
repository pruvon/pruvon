APP_NAME := pruvon
PKG := ./cmd/app
BUILD_DIR := builds
LDFLAGS := -s -w
GOFMT_FILES := $(shell git ls-files '*.go')

.PHONY: help build build-linux build-linux-amd64 build-linux-arm64 fmt test test-race vet lint changelog clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make build        Build local ./pruvon binary' \
		'  make build-linux  Build Linux amd64/arm64 binaries into builds/' \
		'  make fmt          Run gofmt on tracked Go files' \
		'  make test         Run go test ./...' \
		'  make test-race    Run go test -race ./...' \
		'  make vet          Run go vet ./...' \
		'  make lint         Run golangci-lint using repo config' \
		'  make changelog VERSION=x.y.z [PREVIOUS_TAG=vx.y.z]  Regenerate CHANGELOG.md from commit subjects' \
		'  make clean        Remove generated binaries and build artifacts'

build:
	go build -o $(APP_NAME) $(PKG)

build-linux: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(PKG)

build-linux-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(PKG)

fmt:
	gofmt -w $(GOFMT_FILES)

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is not installed"; exit 1; }
	@printf '%s\n' "Using $(shell golangci-lint version | head -1)"
	golangci-lint run --timeout=5m

changelog:
	@test -n "$(VERSION)" || { echo "VERSION is required, e.g. make changelog VERSION=0.1.1"; exit 1; }
	bash scripts/changelog/generate.sh "$(VERSION)" "$(PREVIOUS_TAG)"

clean:
	rm -f $(APP_NAME)
	rm -f $(BUILD_DIR)/$(APP_NAME)-linux-amd64
	rm -f $(BUILD_DIR)/$(APP_NAME)-linux-arm64
	rmdir $(BUILD_DIR) 2>/dev/null || true
