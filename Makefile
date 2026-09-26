APP_NAME := gsm2mqtt
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/legoser/gsm2mqtt/internal/version.Version=$(VERSION) -X github.com/legoser/gsm2mqtt/internal/version.BuildTime=$(BUILD_TIME) -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

.PHONY: build build-small build-noapi build-all build-riscv64 build-mips build-mipsel package-openwrt package-opkg package-apk changelog release-notes test test-cover lint vet docker clean help

## build: Build for current platform
build:
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/gsm2mqtt/

## build-small: Build for current platform and compress with UPX
build-small: build
	@command -v upx >/dev/null 2>&1 || { \
		echo "Error: 'upx' utility is not installed." >&2; \
		echo "UPX is required to compress binaries for resource-constrained targets." >&2; \
		echo "Install UPX via your package manager:" >&2; \
		echo "  - Arch Linux:    sudo pacman -S upx" >&2; \
		echo "  - Debian/Ubuntu: sudo apt install upx-ucl (or upx)" >&2; \
		echo "  - macOS:         brew install upx" >&2; \
		exit 1; \
	}
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

## build-mips: Cross-compile for Linux MIPS big-endian (softfloat, headless no_api + pure TCP no_tls)
build-mips:
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat go build -tags "no_api,no_tls" -trimpath $(LDFLAGS) -o bin/$(APP_NAME)-linux-mips ./cmd/gsm2mqtt/

## build-mipsel: Cross-compile for Linux MIPS little-endian (softfloat, headless no_api + pure TCP no_tls)
build-mipsel:
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -tags "no_api,no_tls" -trimpath $(LDFLAGS) -o bin/$(APP_NAME)-linux-mipsel ./cmd/gsm2mqtt/

## package-openwrt: Build OpenWrt packages (both OPKG and APK for all architectures)
package-openwrt:
	@chmod +x ./scripts/build_openwrt_packages.sh
	./scripts/build_openwrt_packages.sh --type all --arch all --version $(VERSION)

## package-opkg: Build OpenWrt OPKG (.ipk) packages for all architectures
package-opkg:
	@chmod +x ./scripts/build_openwrt_packages.sh
	./scripts/build_openwrt_packages.sh --type ipk --arch all --version $(VERSION)

## package-apk: Build OpenWrt APK (.apk) packages for all architectures
package-apk:
	@chmod +x ./scripts/build_openwrt_packages.sh
	./scripts/build_openwrt_packages.sh --type apk --arch all --version $(VERSION)

## changelog: Auto-generate and record release section into CHANGELOG.md
changelog:
	@command -v python3 >/dev/null 2>&1 || { echo "Error: 'python3' is required to generate changelog." >&2; exit 1; }
	python3 scripts/generate_release_notes.py --update-changelog

## release-notes: Generate formatted release notes for current tag/HEAD
release-notes:
	@command -v python3 >/dev/null 2>&1 || { echo "Error: 'python3' is required to generate release notes." >&2; exit 1; }
	python3 scripts/generate_release_notes.py

## prepare-release: Prepare release cut (update CHANGELOG, commit, tag) [VERSION=vX.Y.Z] [BUMP=patch|minor|major] [FORCE=1]
prepare-release:
	@command -v python3 >/dev/null 2>&1 || { echo "Error: 'python3' is required for release preparation." >&2; exit 1; }
	python3 scripts/prepare_release.py $(if $(VERSION),--version $(VERSION),) $(if $(BUMP),--bump $(BUMP),) $(if $(FORCE),--force,)

## setup-hooks: Configure local Git to use project hooks from .githooks/
setup-hooks:
	@chmod +x .githooks/*
	git config core.hooksPath .githooks
	@echo "✓ Git hooks enabled from .githooks/"


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
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Error: 'golangci-lint' is not installed." >&2; \
		echo "Install it via: https://golangci-lint.run/welcome/install/" >&2; \
		echo "Or run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" >&2; \
		exit 1; \
	}
	golangci-lint run ./...

## docker: Build Docker image
docker:
	@command -v docker >/dev/null 2>&1 || { echo "Error: 'docker' is not installed or not in PATH." >&2; exit 1; }
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
