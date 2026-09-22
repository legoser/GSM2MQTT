# GSM2MQTT

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://go.dev/)

GSM-to-MQTT gateway service for home automation. Connects GSM modems to
Home Assistant and other MQTT-based systems via AT commands.

## Features

- **SMS send/receive** — with Cyrillic support (UCS-2), transliteration, multipart
- **Voice calls** — dial, answer, hangup, DTMF send/receive
- **Delivery reports** — track SMS delivery with timeout escalation
- **Multi-modem** — architecture supports multiple modems
- **MQTT Auto Discovery** — automatic Home Assistant integration
- **Security** — rate limiting, phone number filtering, AT command sanitization
- **Cross-platform** — static binary for Linux x86_64, ARM64

## Supported Modems

| Modem | Interface | Status |
|:---|:---|:---|
| Siemens TC35/MC55/TC65 | COM (RS-232) | Phase 1 |
| SIM800L / SIM900 | UART | Phase 1 |
| Huawei USB 3G/4G | USB (stick mode) | Phase 1 |
| Quectel EC25/EG25 | USB/UART | Phase 2 |

## Quick Start

```bash
# Build
make build

# Configure
cp configs/gsm2mqtt.example.yaml /etc/gsm2mqtt/gsm2mqtt.yaml
# Edit the config file with your settings

# Run
./bin/gsm2mqtt --config /etc/gsm2mqtt/gsm2mqtt.yaml
```

## Docker

```bash
docker-compose -f deployments/docker/docker-compose.yml up -d
```

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

## Building

```bash
# Current platform
make build

# All platforms
make build-all

# Run tests
make test
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
