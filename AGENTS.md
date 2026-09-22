# GSM2MQTT — Agent Instructions

## Project Overview

**GSM2MQTT** is a Go gateway service that connects GSM modems of various types
to home automation systems via the MQTT protocol. The service manages modems
through AT commands and publishes/receives data through MQTT topics.

- **Language:** Go 1.22+
- **License:** Apache 2.0
- **Module:** `github.com/legoser/gsm2mqtt`
- **External dependencies:** Exactly 3 (serial, MQTT, YAML). Do NOT add more.

## Code Quality Requirements

### 1. Single Responsibility Principle

- **One file = one responsibility.** Never create "god files" with 500+ lines.
- **One function = one task.** If a function does two things, split it.
- **One package = one domain.** Packages must have clear boundaries.
- Maximum file length: **300 lines** (excluding tests). If longer — refactor.
- Maximum function length: **50 lines**. If longer — extract helper functions.

### 2. Clean Abstractions

- Define **interfaces** at the consumer side, not the provider side.
- Use **dependency injection** — never create dependencies inside constructors.
- All external I/O (serial, MQTT, filesystem) must be behind interfaces for testability.
- Prefer **composition over inheritance** (Go doesn't have inheritance, but avoid deep embedding).

### 3. Module Isolation

- `internal/` packages must NOT import each other in cycles.
- Dependency direction: `cmd/ → internal/services → internal/modem, internal/mqtt`
- The `modem` package must know NOTHING about MQTT.
- The `mqtt` package must know NOTHING about serial ports.
- Services orchestrate between modem and MQTT layers.

```
cmd/gsm2mqtt/
    ↓
internal/config/         (no deps on other internal packages)
internal/transport/      (no deps on other internal packages)
internal/modem/          (depends on: transport)
internal/sms/            (depends on: modem/at for PDU types only)
internal/security/       (no deps on other internal packages)
internal/mqtt/           (no deps on modem, transport)
internal/services/       (depends on: modem, mqtt, sms, security)
internal/api/            (depends on: services)
```

### 4. Test-First Development (TDD)

- **ALWAYS write tests BEFORE implementation.**
- Tests must be **red** (failing) before writing the production code.
- Tests must be **green** after implementation.
- Test file naming: `*_test.go` in the same package.
- Use table-driven tests where appropriate.
- Mock external dependencies (serial port, MQTT client).
- Integration tests tagged with `//go:build integration`.

### 5. Error Handling

- Always wrap errors with context: `fmt.Errorf("failed to send SMS: %w", err)`
- Never ignore errors silently. Log them or return them.
- Use `errors.Is()` and `errors.As()` for error checking.
- Define sentinel errors in the package that owns the concept.

### 6. Naming Conventions

- Package names: lowercase, single word (`config`, `modem`, `sms`).
- Interface names: describe behavior (`Sender`, `Reader`, `Driver`).
- Exported functions: verb + noun (`SendSMS`, `ParsePDU`).
- Unexported helpers: descriptive, no abbreviations.
- Constants: `CamelCase` for exported, `camelCase` for unexported.

### 7. Documentation

- Every exported type, function, and constant MUST have a doc comment.
- Doc comments start with the name of the thing being documented.
- Package-level doc comment in `doc.go` for every package.
- Non-obvious algorithms must have inline comments explaining WHY, not WHAT.

### 8. Logging

- Use `log/slog` (standard library) for all logging.
- Structured logging with key-value pairs.
- Log levels: `Debug` (AT commands), `Info` (SMS sent/received), `Warn` (retries), `Error` (failures).
- NEVER log sensitive data (SIM PIN, MQTT password, SMS content in production).

### 9. Configuration

- Config format: YAML + environment variable overrides.
- ENV pattern: `GSM2MQTT_<SECTION>_<PARAM>` (uppercase).
- All config values must have sensible defaults.
- Validate config at startup, fail fast with clear error messages.

### 10. Cross-Compilation

- All code must compile with `CGO_ENABLED=0`.
- Target platforms: `linux/amd64`, `linux/arm64`.
- Do NOT use platform-specific code without build tags.
- Test cross-compilation in CI: `GOOS=linux GOARCH=arm64 go build ./...`

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│                   cmd/gsm2mqtt/                     │
│                   (entry point)                     │
└─────────────┬───────────────────────────┬───────────┘
              │                           │
     ┌────────▼─────────┐       ┌────────▼─────────┐
     │internal/services │       │  internal/config │
     │ (orchestration)  │       │  (YAML + ENV)    │
     └──┬─────┬─────┬───┘       └──────────────────┘
        │     │     │
   ┌────▼──┐  │  ┌──▼──────┐
   │ modem │  │  │  mqtt    │
   │(AT,   │  │  │(pub/sub, │
   │drivers)│ │  │discovery)│
   └────┬──┘  │  └──────────┘
        │     │
   ┌────▼──┐ ┌▼─────────┐
   │ sms   │ │ security   │
   │(PDU,  │ │(rate limit,│
   │ translit)││filter)   │
   └────┬──┘ └───────────┘
        │
   ┌────▼──────┐
   │ transport   │
   │(serial port)│
   └────────────┘
```

## Development Workflow

1. Read the task requirements carefully.
2. Write failing tests that verify the requirements.
3. Run tests — confirm they are RED.
4. Implement the minimum code to make tests GREEN.
5. Refactor if needed (keep tests GREEN).
6. Run `go vet ./...` and `go build ./...`.
7. Commit with a meaningful message.

## Key Files

- `cmd/gsm2mqtt/main.go` — Entry point, wiring, signal handling.
- `internal/config/config.go` — Configuration structures.
- `internal/config/loader.go` — YAML + ENV config loader.
- `internal/transport/serial.go` — Serial port abstraction.
- `internal/modem/at/engine.go` — AT command send/receive engine.
- `internal/modem/drivers/driver.go` — Modem driver interface.
- `internal/sms/pdu/encoder.go` — PDU encoder (GSM-7, UCS-2, multipart).
- `internal/sms/pdu/decoder.go` — PDU decoder.
- `internal/sms/translit.go` — Cyrillic transliteration.
- `internal/sms/tracker.go` — SMS delivery report tracker.
- `internal/sms/assembler.go` — Multipart SMS assembler.
- `internal/security/ratelimit.go` — SMS rate limiter.
- `internal/security/filter.go` — Phone number whitelist/blacklist.
- `internal/mqtt/client.go` — MQTT client wrapper.
- `internal/mqtt/discovery.go` — Home Assistant MQTT Auto Discovery.
- `internal/services/sms.go` — SMS service (orchestrates modem ↔ MQTT).
- `internal/services/call.go` — Voice call service.
- `internal/services/status.go` — Modem status monitoring.

## External Dependencies (STRICT — do not add more)

| Dependency | Purpose | Why not stdlib |
|:---|:---|:---|
| `go.bug.st/serial` | Serial port I/O | Requires ioctl/termios, ~1500 LOC |
| `github.com/eclipse/paho.mqtt.golang` | MQTT client | Binary protocol, QoS, reconnect |
| `gopkg.in/yaml.v3` | YAML config parser | YAML spec is 80 pages |

Everything else is implemented using the Go standard library.
