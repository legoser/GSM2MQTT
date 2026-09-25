# GSM2MQTT: Supported Modems & Hardware Guide

This document describes supported GSM/3G/4G hardware, wiring guidelines, baud rate configuration, power requirements, and operator presets for **GSM2MQTT**.

---

## 1. Supported Modem Matrix

| Modem Family | Interfaces | Typical Bandwidth | Voice Support | Recommended Baud | Flow Control | Notes |
|:---|:---|:---:|:---:|:---:|:---:|:---|
| **Siemens TC35 / MC55 / TC65** | RS-232 / UART | 2G (GPRS) | Yes | `9600` – `115200` | Hardware (RTS/CTS) | Industrial workhorse. Very stable, ignores autobauding when fixed. |
| **SIMCom SIM800L / SIM900** | UART (TTL 3.3V/5V) | 2G (GPRS) | Yes (AT+DDET) | `9600` – `115200` | None | Requires high current power supply (2A peak pulses at 3.7V–4.2V). |
| **Huawei 3G/4G USB Sticks** | USB (ttyUSB / ACM) | 3G / LTE | Yes (with voice firmware) | `115200` | None | Must be in "Stick" (modem) mode, not "HiLink" (router) mode. |
| **Quectel EC25 / EG25** | USB ACM / UART | LTE Cat 4 | Yes (PCM / VoLTE) | `115200` | Hardware (RTS/CTS) | High-speed modern LTE module. Excellent signal diagnostics. |
| **Generic AT Modems** | Serial / USB | Varies | Optional | `9600` – `115200` | None / Hardware | Adheres to standard 3GPP TS 27.005 / TS 27.007 specifications. |

---

## 2. Hardware Wiring & Power Supply Requirements

### 2.1 SIMCom SIM800L
> [!CAUTION]
> **Power Supply Warning:** The SIM800L module cannot be powered directly from a Raspberry Pi or USB 5V rail. During GSM transmission bursts, it consumes up to **2.0A peak current**.
- **Voltage Range**: `3.7V – 4.2V` (optimal: `4.0V`).
- **Capacitor**: Place a low-ESR electrolytic capacitor (`1000 µF – 2200 µF`) directly across the module's `VCC` and `GND` pins.
- **Logic Level**: While the module's UART tolerates 3.3V, use a level shifter or 1kΩ/2kΩ resistor divider when connecting to 5V microcontrollers.

### 2.2 Siemens TC35 / MC55
- **Power**: Industrial standard `9V – 15V DC` (minimum 1A).
- **Interface**: Connect via standard RS-232 serial cable or USB-to-RS232 adapter (FTDI / CH340 / CP2102).
- **RTS/CTS**: Highly recommended to enable `flow_control: "hardware"` in `gsm2mqtt.yaml` to prevent serial buffer overruns during high-speed traffic.

### 2.3 Huawei USB Modems (E1550, E173, E3131, E3531)
- **Stick Mode vs HiLink**:
  - **Stick Mode**: Presents virtual serial ports (`/dev/ttyUSB0`, `/dev/ttyUSB1`, `/dev/ttyUSB2`). Supported by GSM2MQTT out of the box.
  - **HiLink Mode**: Presents an RNDIS/Ethernet interface. Must be switched to modem mode via `usb_modeswitch` or cross-flashed to stick firmware.
- **Voice Capabilities**: Some Huawei modems require voice unlock codes (`AT^SYSCFG` or DC-Unlocker) to enable incoming/outgoing voice calls.

---

## 3. Operator Presets & Balance Management

GSM2MQTT features built-in operator presets for automatic USSD balance checking, balance extraction via regular expressions, and quota monitoring:

| Preset Name | Carrier | USSD Code | Balance Extraction Pattern | Package Info Code |
|:---|:---|:---:|:---|:---:|
| `mts` | МТС | `*100#` | `(?i)(?:баланс\|balance)[:\s]*([\d\.,]+)` | `*100*1#` |
| `megafon` | МегаФон | `*100#` | `(?i)(?:баланс\|balance)[:\s]*([\d\.,]+)` | `*558#` |
| `beeline` | Билайн | `*102#` | `(?i)(?:баланс\|balance)[:\s]*([\d\.,]+)` | `*106#` |
| `tele2` | Tele2 / t2 | `*105#` | `(?i)(?:баланс\|balance)[:\s]*([\d\.,]+)` | `*107#` |
| `generic` / `custom` | Custom | Configurable | User-supplied regular expression | Configurable |

### Configuration Example
```yaml
tariff:
  operator_preset: "mts"
  balance_ussd: "*100#"
  check_interval: 24h
  min_balance_alert: 50.0   # Generates warning alert if balance drops below 50.00
  auto_check_on_error: true # Dispatches USSD check if SMS delivery fails
  packages:
    sms_limit: 500          # Monthly bundled SMS
    call_minutes_limit: 100 # Monthly voice minutes
    data_limit_mb: 1000     # Monthly data allowance
    reset_day: 1            # Day of the month when counters reset
```

---

## 4. Troubleshooting & Diagnostic Commands

Run these standard AT commands via your favorite terminal emulator (`minicom`, `screen`, `picocom`) or through GSM2MQTT's REST API / raw MQTT channel to verify modem health:

```bash
# Check basic connectivity
AT
# Output: OK

# Check SIM readiness (must return READY)
AT+CPIN?
# Output: +CPIN: READY

# Check network registration (first digit 0 or 1, second digit 1 or 5)
AT+CREG?
# Output: +CREG: 0,1 (Home network) or +CREG: 0,5 (Roaming)

# Check signal quality (first value: 0..31, second: BER)
AT+CSQ
# Output: +CSQ: 22,0 (22 = -69 dBm, Excellent)

# Check operator name
AT+COPS?
# Output: +COPS: 0,0,"MTS"
```
