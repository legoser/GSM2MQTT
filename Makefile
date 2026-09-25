APP_NAME := gsm2mqtt
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/legoser/gsm2mqtt/internal/version.Version=$(VERSION) -X github.com/legoser/gsm2mqtt/internal/version.BuildTime=$(BUILD_TIME) -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

.PHONY: build build-all build-riscv64 package-openwrt package-opkg package-apk changelog release-notes test test-cover lint vet docker clean help

## build: Build for current platform
build:
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/gsm2mqtt/

## build-small: Build for current platform and compress with UPX
build-small: build
	upx --best --lzma bin/$(APP_NAME)

## build-noapi: Build for current platform without HTTP API (smaller binary)
build-noapi:
	CGO_ENABLED=0 go build -tags no_api -trimpath $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/gsm2mqtt/

## build-all: Cross-compile for all target platforms (amd64, arm64, riscv64)
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./cmd/gsm2mqtt/
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath $(LDFLAGS) -o bin/$(APP_NAME)-linux-arm64 ./cmd/gsm2mqtt/
	CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build -trimpath $(LDFLAGS) -o bin/$(APP_NAME)-linux-riscv64 ./cmd/gsm2mqtt/

## build-riscv64: Cross-compile for Linux RISC-V 64-bit
build-riscv64:
	CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build -trimpath $(LDFLAGS) -o bin/$(APP_NAME)-linux-riscv64 ./cmd/gsm2mqtt/

## package-openwrt: Build OpenWrt packages (both OPKG and APK for all architectures)
package-openwrt:
	./scripts/build_openwrt_packages.sh --type all --arch all --version $(VERSION)

## package-opkg: Build OpenWrt OPKG (.ipk) packages for all architectures
package-opkg:
	./scripts/build_openwrt_packages.sh --type ipk --arch all --version $(VERSION)

## package-apk: Build OpenWrt APK (.apk) packages for all architectures
package-apk:
	./scripts/build_openwrt_packages.sh --type apk --arch all --version $(VERSION)

## changelog: Auto-generate and record release section into CHANGELOG.md
changelog:
	python3 scripts/generate_release_notes.py --update-changelog

## release-notes: Generate formatted release notes for current tag/HEAD
release-notes:
	python3 scripts/generate_release_notes.py

## test: Run all tests
test:
	go test -v -race -count=1 ./...

## test-cover: Run tests with coverage report
test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## vet: Run Go vet
vet:
	go vet ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## docker: Build Docker image
docker:
	docker build -t $(APP_NAME):$(VERSION) -f deployments/docker/Dockerfile .

## docker-up: Start services with Docker Compose
docker-up:
	docker compose up -d --build

## docker-down: Stop Docker Compose services
docker-down:
	docker compose down

## clean: Remove build artifacts
clean:
	rm -rf bin/ dist/ coverage.out coverage.html

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
