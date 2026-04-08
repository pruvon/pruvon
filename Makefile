APP_NAME := pruvon
PKG := ./cmd/app
BUILD_DIR := builds
DIST_DIR := dist
LDFLAGS := -s -w
GOFMT_FILES := $(shell git ls-files '*.go')

.PHONY: help build build-linux-amd64 build-linux-arm64 dist dist-linux-amd64 dist-linux-arm64 dist-archives dist-checksums fmt test test-race vet lint changelog release clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make build        Build Linux amd64/arm64 binaries into builds/' \
		'  make fmt          Run gofmt on tracked Go files' \
		'  make test         Run go test ./...' \
		'  make test-race    Run go test -race ./...' \
		'  make vet          Run go vet ./...' \
		'  make lint         Run golangci-lint using repo config' \
		'  make changelog VERSION=x.y.z [PREVIOUS_TAG=vx.y.z]  Regenerate CHANGELOG.md from commit subjects' \
		'  make release VERSION=vx.y.z [PREVIOUS_TAG=vx.y.z] [NOTES_FILE=path]  Build, tag, and publish a GitHub release from local machine' \
		'  make clean        Remove generated binaries and build artifacts'

build: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(PKG)

build-linux-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(PKG)

dist: dist-linux-amd64 dist-linux-arm64 dist-archives dist-checksums

dist-linux-amd64:
	@test -n "$(VERSION)" || { echo "VERSION is required, e.g. make dist VERSION=v0.1.1"; exit 1; }
	mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS) -X main.PruvonVersion=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 $(PKG)

dist-linux-arm64:
	@test -n "$(VERSION)" || { echo "VERSION is required, e.g. make dist VERSION=v0.1.1"; exit 1; }
	mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS) -X main.PruvonVersion=$(VERSION)" -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 $(PKG)

dist-archives:
	COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 tar -czf $(DIST_DIR)/$(APP_NAME)-linux-amd64.tar.gz -C $(DIST_DIR) $(APP_NAME)-linux-amd64
	COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 tar -czf $(DIST_DIR)/$(APP_NAME)-linux-arm64.tar.gz -C $(DIST_DIR) $(APP_NAME)-linux-arm64

dist-checksums:
	@if command -v sha256sum >/dev/null 2>&1; then \
		cd $(DIST_DIR) && sha256sum $(APP_NAME)-linux-amd64.tar.gz $(APP_NAME)-linux-arm64.tar.gz > checksums.txt; \
	elif command -v shasum >/dev/null 2>&1; then \
		cd $(DIST_DIR) && shasum -a 256 $(APP_NAME)-linux-amd64.tar.gz $(APP_NAME)-linux-arm64.tar.gz > checksums.txt; \
	else \
		echo "Error: neither sha256sum nor shasum is available"; exit 1; \
	fi

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

release:
	@test -n "$(VERSION)" || { echo "VERSION is required, e.g. make release VERSION=v0.1.1"; exit 1; }
	bash scripts/release/local.sh "$(VERSION)" "$(PREVIOUS_TAG)" "$(NOTES_FILE)"

clean:
	rm -f $(BUILD_DIR)/$(APP_NAME)-linux-amd64
	rm -f $(BUILD_DIR)/$(APP_NAME)-linux-arm64
	rmdir $(BUILD_DIR) 2>/dev/null || true
	rm -f $(DIST_DIR)/$(APP_NAME)-linux-amd64
	rm -f $(DIST_DIR)/$(APP_NAME)-linux-arm64
	rm -f $(DIST_DIR)/$(APP_NAME)-linux-amd64.tar.gz
	rm -f $(DIST_DIR)/$(APP_NAME)-linux-arm64.tar.gz
	rm -f $(DIST_DIR)/checksums.txt
	rmdir $(DIST_DIR) 2>/dev/null || true
