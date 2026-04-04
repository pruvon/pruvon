APP_NAME := pruvon
PKG := ./cmd/app
BUILD_DIR := builds
LDFLAGS := -s -w

.PHONY: help build build-linux build-linux-amd64 build-linux-arm64 test vet clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make build        Build local ./pruvon binary' \
		'  make build-linux  Build Linux amd64/arm64 binaries into builds/' \
		'  make test         Run go test ./...' \
		'  make vet          Run go vet ./...' \
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

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(APP_NAME)
	rm -rf $(BUILD_DIR)
