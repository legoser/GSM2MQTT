# GSM2MQTT: MQTT Topic & Protocol Reference

This document provides a complete reference for all MQTT topics, JSON payload schemas, command controls, and telemetry messages published and received by **GSM2MQTT**.

---

## 1. Topic Hierarchy Overview

By default, all topics are prefixed with `gsm2mqtt` (customizable via `mqtt.topic_prefix` in YAML):

```
gsm2mqtt/
├── bridge/
│   ├── state                       # Global gateway availability ("online" / "offline")
│   └── version                     # Current gateway version string
├── security/
│   ├── rate_limit_hit              # Alert: Outbound message rejected by rate limiter
│   └── blocked                     # Alert: Inbound message blocked by security filter
└── modem/
    └── {modem_id}/
        ├── status                  # Detailed hardware & registration state
        ├── health                  # Consolidated operational status (ready, degraded, error)
        ├── alert                   # High-priority alerts (hardware failure, balance exhausted)
        ├── diagnostic              # Diagnostic reports after operation failures
        ├── accounting/
        │   ├── status              # Real-time balance, SMS quotas, and data counters
        │   └── alert               # Quota limit warnings (e.g. 90% reached, low balance)
        ├── sms/
        │   ├── send                # [SUB] Command to transmit an SMS message
        │   ├── received            # [PUB] Event: Inbound SMS arrived
        │   └── delivery_report     # [PUB] Event: Network delivery receipt confirmed
        ├── call/
        │   ├── dial                # [SUB] Command to initiate an outgoing voice call
        │   ├── hangup              # [SUB] Command to terminate or reject a call
        │   ├── dtmf                # [SUB] Command to transmit DTMF tones
        │   └── status              # [PUB] Real-time call state (idle, dialing, incoming, active)
        ├── ussd/
        │   ├── send                # [SUB] Command to dispatch a USSD query
        │   └── response            # [PUB] Event: Decoded USSD response from network
        └── command/
            ├── raw                 # [SUB] Direct AT command injection (requires allow_raw_at)
            └── response            # [PUB] Direct AT command terminal response
```

---

## 2. Inbound Commands (Controlling Modems)

### 2.1 Send SMS (`gsm2mqtt/modem/{id}/sms/send`)
Instructs the specified modem to compose and transmit an SMS.

**Payload:**
```json
{
  "to": "+79001234567",
  "text": "Alarm activated in Living Room!",
  "flash": false
}
```

| Field | Type | Required | Description |
|:---|:---|:---:|:---|
| `to` | string | **Yes** | Destination phone number in E.164 format. |
| `text` | string | **Yes** | Message body. Supports Unicode / Cyrillic (automatically encoded via UCS-2) and standard GSM-7. Multipart messages are split and joined transparently. |
| `flash` | boolean | No | If `true`, sends a Class 0 (Flash) SMS that displays immediately on the recipient's screen without saving. |

---

### 2.2 Make Voice Call (`gsm2mqtt/modem/{id}/call/dial`)
Initiates an outgoing voice call.

**Payload:**
```json
{
  "to": "+79001234567",
  "timeout_sec": 30
}
```

---

### 2.3 Hang Up Call (`gsm2mqtt/modem/{id}/call/hangup`)
Terminates the active call or rejects an incoming call.

**Payload:** Empty payload or `{}`.

---

### 2.4 Send DTMF Tones (`gsm2mqtt/modem/{id}/call/dtmf`)
Transmits DTMF tones during an active voice session (e.g. to navigate automated interactive voice menus).

**Payload:**
```json
{
  "tones": "1234#"
}
```

---

### 2.5 Dispatch USSD Query (`gsm2mqtt/modem/{id}/ussd/send`)
Executes an interactive or standalone USSD code.

**Payload:**
```json
{
  "code": "*100#"
}
```

---

### 2.6 Raw AT Command Injection (`gsm2mqtt/modem/{id}/command/raw`)
Sends arbitrary AT commands directly to the modem serial interface.
*(Note: Ignored unless `security.allow_raw_at: true` is enabled in YAML).*

**Payload:** Raw text, e.g. `AT+CSQ` or `AT+COPS?`.

---

## 3. Outbound Telemetry & Status Events

### 3.1 Modem Health Channel (`gsm2mqtt/modem/{id}/health`)
A consolidated operational health indicator designed for dashboards and Home Assistant status cards.

**Payload:**
```json
{
  "state": "ready",
  "message": "All systems operational",
  "sim_status": "READY",
  "registered": true,
  "signal_dbm": -73,
  "signal_level": "good",
  "balance": 152.40,
  "currency": "RUB",
  "sms_remaining": 412,
  "updated_at": "2026-09-25T14:30:00Z"
}
```

#### State Definitions
* **`ready`**: SIM unlocked, registered to GSM carrier, signal strength adequate, positive balance.
* **`degraded`**: Modem is functioning, but operating under adverse conditions (weak signal < -100 dBm, balance below warning threshold, or monthly SMS quota nearly exhausted).
* **`not_ready`**: Unregistered from network, roaming denied, or SIM requires PIN/PUK.
* **`error`**: Hardware malfunction, serial interface disconnection, or unresponsive AT command interface.
* **`disconnected`**: Physical serial interface disconnected, port not found, or modem unpowered. Resets active modems count in Home Assistant to 0 and clears stale `ready` states.

---

### 3.2 Inbound SMS Notification (`gsm2mqtt/modem/{id}/sms/received`)
Published as soon as an incoming SMS arrives on the modem.

**Payload:**
```json
{
  "from": "+79001234567",
  "text": "Water sensor: normal",
  "timestamp": "2026-09-25T14:32:05Z",
  "modem_id": "siemens_tc35",
  "multipart": false
}
```

---

### 3.3 SMS Delivery Report (`gsm2mqtt/modem/{id}/sms/delivery_report`)
Published when the mobile network confirms successful delivery of an outbound SMS.

**Payload:**
```json
{
  "message_ref": 14,
  "recipient": "+79001234567",
  "status": "delivered",
  "discharge_time": "2026-09-25T14:32:15Z"
}
```

---

### 3.4 Tariff & Balance Accounting (`gsm2mqtt/modem/{id}/accounting/status`)
Tracks operator balance and monthly package usage.

**Payload:**
```json
{
  "balance": 152.40,
  "currency": "RUB",
  "sms_month_used": 88,
  "sms_month_limit": 500,
  "sms_remaining": 412,
  "call_minutes_used": 14.5,
  "call_minutes_limit": 100,
  "data_bytes_used": 1048576,
  "last_checked": "2026-09-25T12:00:00Z"
}
```

---

### 3.5 Automated Diagnostic Reports (`gsm2mqtt/modem/{id}/diagnostic`)
Published whenever an error occurs and the automated self-healing failure tree runs.

**Payload:**
```json
{
  "event": "sms_send_failed",
  "error": "+CMS ERROR: 38",
  "diagnosis": "Network out of order / temporary carrier failure",
  "sim_status": "READY",
  "registration": "Registered, home network",
  "csq": 18,
  "signal_dbm": -77,
  "balance": 152.40,
  "action_taken": "Switched to fallback modem in pool"
}
```

---

### 3.6 Urgent Alerts (`gsm2mqtt/modem/{id}/alert`)
High-priority alerts that can be wired directly to smartphone push notifications:

**Payload:**
```json
{
  "level": "warning",
  "type": "low_balance",
  "message": "SIM card balance (35.50 RUB) dropped below threshold (50.00 RUB)",
  "timestamp": "2026-09-25T14:35:00Z"
}
```

---

## 4. Global Bridge Telemetry

* **`gsm2mqtt/bridge/state`**: Published as `online` when the service connects. Configured with MQTT Last Will and Testament (LWT) to publish `offline` if the gateway process unexpectedly terminates or loses power.
* **`gsm2mqtt/security/rate_limit_hit`**: Published when an automation or user sends SMS faster than allowed by the security policy.
