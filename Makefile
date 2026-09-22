APP_NAME := gsm2mqtt
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

.PHONY: build build-all build-riscv64 test test-cover lint vet docker clean help

## build: Build for current platform
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/gsm2mqtt/

## build-all: Cross-compile for all target platforms (amd64, arm64, riscv64)
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./cmd/gsm2mqtt/
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-arm64 ./cmd/gsm2mqtt/
	CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-riscv64 ./cmd/gsm2mqtt/

## build-riscv64: Cross-compile for Linux RISC-V 64-bit
build-riscv64:
	CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-riscv64 ./cmd/gsm2mqtt/

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
	rm -rf bin/ coverage.out coverage.html

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
