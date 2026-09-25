# GSM2MQTT

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://go.dev/)

GSM-to-MQTT gateway service for home automation. Connects GSM modems to
Home Assistant and other MQTT-based systems via AT commands.

## Features

- **SMS send/receive** — with Cyrillic support (UCS-2), transliteration, multipart, delivery reports
- **Voice calls** — dial, answer, hangup, DTMF send/receive
- **Multi-modem Pool** — load balancing (round-robin, failover, best-signal, operator-match) with SIM redundancy
- **Operator Presets & Tariff Accounting** — MTS, Megafon, Beeline, Tele2 balance parsing, daily/monthly SMS quotas
- **Automated Diagnostics & Alerts** — failure tree inspection (SIM, network, signal, balance) with MQTT alerts
- **Prometheus Metrics** — embedded exposition format without external dependencies (`GET /metrics`)
- **Embedded Web Dashboard & REST API** — status inspection, signal levels, SMS and USSD forms
- **AT HTTP Client** — direct HTTP GET/POST requests via modem stack (`AT+HTTP*`) without PPP
- **MQTT Auto Discovery** — seamless Home Assistant integration
- **Security** — sliding-window rate limiting, phone number filtering, AT command sanitization
- **Cross-platform** — zero-CGO static binaries for Linux `amd64`, `arm64`, and `riscv64`
- **Lightweight & Embedded Ready** — optional headless build (`-tags no_api`) and automated UPX compression (`make build-small`) for storage-constrained OpenWrt routers

## Supported Modems

| Modem | Interface | Status |
|:---|:---|:---|
| Siemens TC35/MC55/TC65 | COM (RS-232) | Supported |
| SIM800L / SIM900 | UART | Supported |
| Huawei USB 3G/4G | USB (stick mode) | Supported |
| Quectel EC25/EG25 | USB/UART | Supported (Generic/AT) |
| Generic AT Modems | UART / USB ACM | Supported |

## Quick Installation (One-Liner)

Install or upgrade GSM2MQTT with a single command on **Linux** (Debian, Ubuntu, Raspberry Pi OS, Armbian, Arch, Fedora) or **OpenWrt** (Raspberry Pi, x86_64, MT7621, etc.):

### Linux (systemd / OpenRC)
```bash
curl -fsSL https://raw.githubusercontent.com/legoser/gsm2mqtt/main/scripts/install.sh | sudo sh
```

### OpenWrt (OPKG / APK / Standalone procd)
```sh
sh -c "$(wget --no-check-certificate -qO- https://raw.githubusercontent.com/legoser/gsm2mqtt/main/scripts/install.sh)"
```

The installer automatically:
- Detects the CPU architecture (`x86_64`, `aarch64_cortex-a53`, `aarch64_cortex-a72`, `armv7`, `mipsel`, `riscv64`, etc.).
- Downloads the latest release package (`.ipk` / `.apk`) or standalone binary for your device.
- Preserves existing configuration files in `/etc/gsm2mqtt/gsm2mqtt.yaml`.
- Registers and enables the service for autostart (`systemd` or OpenWrt `procd`).
- Detects connected USB/Serial modems (`/dev/ttyUSB*`, `/dev/ttyACM*`) and provides ready-to-run commands.

---

## Manual Installation from Release

Download the archive for your platform from the **Releases** page
(available both in Forgejo and on GitHub):

```
gsm2mqtt-v1.2.3-linux-amd64.tar.gz    # x86_64 server
gsm2mqtt-v1.2.3-linux-arm64.tar.gz    # Raspberry Pi 4/5, ARM server
gsm2mqtt-v1.2.3-linux-riscv64.tar.gz  # RISC-V
checksums.txt                         # SHA-256 of all archives
```

```bash
# Verify integrity, unpack, check version
sha256sum -c checksums.txt
tar -xzf gsm2mqtt-v1.2.3-linux-amd64.tar.gz
./gsm2mqtt --version
# gsm2mqtt v1.2.3 (built 2026-...)

# New version = new tag: git tag v1.2.4 && git push origin v1.2.4
```

## Docker

```bash
docker-compose -f deployments/docker/docker-compose.yml up -d
```

## OpenWrt (OPKG & APK)

GSM2MQTT provides ready-to-use packages for OpenWrt routers with `procd` service integration:

- **OPKG (`.ipk`)** for OpenWrt 23.05 and earlier:
  ```bash
  opkg install gsm2mqtt_<version>_<arch>.ipk
  ```
- **APK (`.apk`)** for OpenWrt 25.12+ and snapshots:
  ```bash
  apk add --allow-untrusted ./gsm2mqtt-<version>-<arch>.apk
  ```

Manage the service via `procd`:
```bash
/etc/init.d/gsm2mqtt start
/etc/init.d/gsm2mqtt status
```
Configuration is stored in `/etc/gsm2mqtt/gsm2mqtt.yaml` and `/etc/config/gsm2mqtt`.
For detailed architecture tables and feed integration, see [OpenWrt Deployment Guide](docs/openwrt.md).

## MQTT Topics

```
gsm2mqtt/
├── status                          # Service online/offline (LWT)
└── modem/{modem_id}/
    ├── status                      # Modem status JSON
    ├── signal                      # Signal strength
    ├── operator                    # Operator name
    ├── sms/
    │   ├── received                # Incoming SMS
    │   ├── send                    # ← Send SMS command
    │   ├── sent                    # Send confirmation
    │   └── status                  # Delivery report events
    ├── call/
    │   ├── incoming                # Incoming call (caller ID)
    │   ├── dial                    # ← Make call command
    │   ├── hangup                  # ← Hangup command
    │   └── dtmf                    # DTMF events
    └── ussd/
        ├── send                    # ← USSD request
        └── response                # USSD response
```

## REST API & Web Dashboard

The embedded HTTP server provides an interactive Web UI (`http://localhost:8088/` in Docker) and a REST API.

### Authentication & Access Control

Protected endpoints require a Bearer token:
```bash
curl -H "Authorization: Bearer <your-token>" http://localhost:8088/api/modems
```

> [!IMPORTANT]
> If `api.token` is left empty in configuration, all mutating and sensitive `/api/*` endpoints return `403 Forbidden` (`{"error":"forbidden","message":"API token is required for this endpoint"}`).
> The monitoring endpoints `GET /metrics` (Prometheus) and `GET /health` always remain publicly accessible without a token.

### 1. Direct AT Commands (`POST /api/at/send`)
Execute arbitrary AT commands directly on any connected modem:
```bash
curl -X POST http://localhost:8088/api/at/send \
  -H "Content-Type: application/json" \
  -d '{"modem_id": "huawei_e1550", "command": "AT+CSQ"}'
```
Response:
```json
{
  "reply": "+CSQ: 21,99",
  "success": true
}
```

### 2. Send SMS (`POST /api/sms/send`)
Supports international (`+7...`) and national (`8...`) numbers, automated PDU encoding (GSM-7 / UCS-2 Cyrillic), and multipart message splitting:
```bash
curl -X POST http://localhost:8088/api/sms/send \
  -H "Content-Type: application/json" \
  -d '{
    "modem_id": "huawei_e1550",
    "to": "+79991234567",
    "text": "Hello from GSM2MQTT!"
  }'
```

### 3. Read Stored Inbox (`GET /api/sms/inbox`)
Retrieve persistent received messages:
```bash
curl http://localhost:8088/api/sms/inbox
```

### 4. USSD Requests (`POST /api/ussd/send`)
Query balance and mobile services:
```bash
curl -X POST http://localhost:8088/api/ussd/send \
  -H "Content-Type: application/json" \
  -d '{"modem_id": "huawei_e1550", "code": "*100#"}'
```

### 5. Voice Calls (`POST /api/call/dial` & `/api/call/hangup`)
Dial a phone number (with automated firmware voice capability pre-check):
```bash
# Dial
curl -X POST http://localhost:8088/api/call/dial \
  -H "Content-Type: application/json" \
  -d '{"modem_id": "huawei_e1550", "number": "+79991234567"}'

# Hang up
curl -X POST http://localhost:8088/api/call/hangup \
  -H "Content-Type: application/json" \
  -d '{"modem_id": "huawei_e1550"}'
```

### 6. Modems Telemetry (`GET /api/modems`)
```bash
curl http://localhost:8088/api/modems
```

## Useful AT Commands (Huawei Tuning)

| Command | Description | Purpose |
|:---|:---|:---|
| `AT^U2DIAG=0` | "Modem Only" mode | Disables virtual CD-ROM and microSD reader in NVRAM. Prevents USB mode flapping and lowers idle power draw. |
| `AT^SYSCFG=14,2,3FFFFFFF,2,4` | Force 3G only (WCDMA) | 3G uses continuous modulation (max 0.25W) without the 2A/2W TDMA current spikes of 2G GSM, preventing USB brownouts. |
| `AT^SYSCFG=2,2,3FFFFFFF,2,4` | Auto 2G/3G mode | Returns modem to automatic network standard selection. |
| `AT^CVOICE?` | Check voice capability | `^CVOICE:0` means voice calls are enabled; `^CVOICE:1` means voice is disabled by operator firmware (data-only). |
| `AT+CSCA?` | Query SMS Service Center | Displays the configured SMSC address. |
| `AT+CSQ` | Query signal strength | Returns RSSI (0..31) and BER. Values >= 15 indicate good reception. |

## SIMCom SIM800 / SIM900 Setup Guide

### 1. Wiring & Electrical Requirements

> [!IMPORTANT]
> **SIM800 is extremely sensitive to voltage drops!** During 2G TDMA transmission bursts, current draw surges to **2.0 A**.
> Powering the module directly from a USB-UART 5V or 3.3V pin will cause brownouts and continuous boot loops (`UNDER-VOLTAGE WARNNING` / `RDY`).

* **Power Supply (VCC)**: Use a dedicated step-down DC-DC converter (e.g. LM2596, MP1584) or a 1S Li-Ion / LiPo battery providing **3.7 V – 4.4 V** (optimal: **4.0 V**).
* **Capacitor**: Place a **1000 µF – 2200 µF Low-ESR electrolytic capacitor** directly across `VCC` and `GND` as close to the SIM800 module pins as possible.
* **Common Ground (GND)**: `GND` of the external power supply, SIM800, and USB-UART adapter **must be connected together**.
* **UART Lines (RX/TX)**:
  * SIM800 `TXD` → USB-UART `RXD`
  * SIM800 `RXD` → USB-UART `TXD` *(If using a 5V USB-UART adapter, use a 1kΩ / 2kΩ resistive voltage divider on the module's RXD pin, as SIM800 GPIO is 2.8V–3.3V logic)*.
* **PWRKEY** (if present on the board): Pulse to `GND` for 1–2 seconds to turn the module on (many SIM800L red boards have PWRKEY permanently tied to GND).

### 2. Useful AT Commands (SIM800 / SIM900)

| Command | Description | Purpose |
|:---|:---|:---|
| `AT+CBC` | Query battery & voltage | Returns `+CBC: <bcs>,<bcl>,<bcv>` where `<bcv>` is the supply voltage in mV (e.g. `4120` = 4.12V). |
| `AT+CSCLK=0` | Disable sleep mode | Prevents the module UART from going into power-save sleep. |
| `AT+CFUN=1` | Full functionality | Ensures radio transceiver and SIM card interface are fully powered. |
| `AT+CLVL=80` | Speaker volume | Adjusts analog voice volume (range: 0..100). |
| `AT+CMIC=0,10` | Microphone gain | Adjusts microphone pre-amplifier gain (channel 0, gain: 0..15). |
| `AT+IPR=115200` | Fix baud rate | Sets fixed UART baud rate and saves to NVRAM (disables autobauding). |
| `AT&W` | Save configuration | Writes current profile settings to non-volatile memory. |


## Building

GSM2MQTT provides multiple build targets tailored for different use cases and deployment targets:

| Make Target | Build Flags | Description | Recommended For |
|:---|:---|:---|:---|
| `make build` | `-trimpath -ldflags "-s -w"` | Standard full-featured build (~8.3 MB) | Servers, Docker, Raspberry Pi |
| `make build-small` | `build` + `upx --best --lzma` | Full-featured build compressed using UPX (~2.5 MB) | OpenWrt routers with limited Flash storage |
| `make build-noapi` | `-tags no_api -trimpath ...` | Headless gateway without HTTP server / Web UI (~7.6 MB uncompressed, ~2.2 MB with UPX) | Dedicated headless gateways, routers with very tight RAM/Flash |
| `make build-all` | Cross-compilation | Builds standard binaries for `amd64`, `arm64`, and `riscv64` | Multi-arch distributions |

### Build Commands

```bash
# Standard build for current platform
make build

# Ultra-compact build using UPX compression (requires upx installed)
make build-small

# Headless build without HTTP API server (saves binary size and RAM)
make build-noapi

# Headless build compressed with UPX
make build-noapi && upx --best --lzma bin/gsm2mqtt

# Cross-compile standard binaries for all architectures
make build-all

# Run test suite & linters
make test
make vet
```

## Documentation

Comprehensive documentation guides are available in the [`docs/`](docs/) directory:

- 🏗️ **[Architecture & System Design](docs/architecture.md)** — Core components, multi-modem pool routing, state machine, and 5-layer security model.
- 📡 **[MQTT Topics & Protocol Reference](docs/mqtt-topics.md)** — Full topic hierarchy, JSON command schemas, and telemetry events.
- 📟 **[Supported Modems & Hardware Guide](docs/modems.md)** — Wiring diagrams, power supply requirements, and operator tariff presets.
- 🏠 **[Home Assistant Integration](docs/home-assistant-integration.md)** — Auto-Discovery setup, sensors, and notification automations.
- 📦 **[OpenWrt Deployment Guide](docs/openwrt.md)** — Standalone package installation (`.ipk` and `.apk`) and procd service management.
- 🚀 **[Developer & Release Workflow Guide](docs/development-and-release-guide.md)** — Commit conventions, automated Git hooks, and release cut workflows.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
