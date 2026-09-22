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

## Supported Modems

| Modem | Interface | Status |
|:---|:---|:---|
| Siemens TC35/MC55/TC65 | COM (RS-232) | Supported |
| SIM800L / SIM900 | UART | Supported |
| Huawei USB 3G/4G | USB (stick mode) | Supported |
| Quectel EC25/EG25 | USB/UART | Supported (Generic/AT) |
| Generic AT Modems | UART / USB ACM | Supported |

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
