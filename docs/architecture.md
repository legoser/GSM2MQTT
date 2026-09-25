# GSM2MQTT: Architecture & System Design

This document details the internal architecture, component design, multi-modem routing, and security model of **GSM2MQTT** for users and third-party developers.

---

## 1. System Overview

GSM2MQTT is a high-performance, concurrent gateway written in Go that bridges physical GSM/3G/4G modems with MQTT-based smart home systems (such as Home Assistant, Node-RED, and OpenHAB).

```mermaid
flowchart TB
    subgraph Hardware["GSM Modem Layer"]
        M1["Siemens TC35 / MC55\n(RS-232 / UART)"]
        M2["SIMCom SIM800 / 900\n(UART)"]
        M3["Huawei USB Stick\n(ttyUSB / ACM)"]
        M4["Quectel EC25 / EG25\n(UART / USB)"]
    end

    subgraph Core["GSM2MQTT Gateway Core"]
        direction TB
        TRANSPORT["Serial Transport Layer\n(go.bug.st/serial + CTS/RTS)"]
        AT_ENGINE["AT Engine & Parser\n(Concurrency-Safe, Timeouts)"]
        DRIVERS["Vendor Modem Drivers\n(Auto-Detection, AT Extensions)"]
        POOL["Modem Pool Manager\n(Failover, Round-Robin, Best-Signal)"]
        TARIFF["Tariff & Accounting Manager\n(Quota tracking, Balance parsing)"]
        DIAG["Auto-Diagnostics Engine\n(Failure Tree, Self-Healing)"]
        SEC["Security & Rate Limiting\n(E.164 filter, AT sanitizer)"]
    end

    subgraph Integrations["Integration Interfaces"]
        MQTT_CLIENT["MQTT Client\n(Paho, QoS 0/1/2, TLS, LWT)"]
        HA_DISC["Home Assistant\nMQTT Auto-Discovery"]
        HTTP_API["Embedded REST API\n(Metrics, Web UI, Bearer Auth)"]
    end

    Hardware <--> TRANSPORT
    TRANSPORT <--> AT_ENGINE
    AT_ENGINE <--> DRIVERS
    DRIVERS <--> POOL
    POOL <--> TARIFF
    POOL <--> DIAG
    POOL <--> SEC
    SEC <--> MQTT_CLIENT
    POOL <--> HTTP_API
    MQTT_CLIENT <--> HA_DISC
```

---

## 2. Multi-Layer Security Architecture

GSM2MQTT operates under a strict defense-in-depth model across 5 concentric security boundaries:

```mermaid
flowchart TB
    subgraph L1["Layer 1: Transport Security"]
        TLS["MQTT over TLS (Port 8883)\nMutual TLS (mTLS) with client certificates"]
        HTTPS["REST API TLS / HTTPS (Optional)"]
    end

    subgraph L2["Layer 2: Network Authentication"]
        MQTT_AUTH["MQTT Username/Password Verification"]
        API_TOKEN["REST API Bearer Token Authentication"]
        IP_FILTER["CIDR IP Allowlist (e.g. 127.0.0.1, 192.168.1.0/24)"]
    end

    subgraph L3["Layer 3: Broker Authorization (ACL)"]
        ACL["Mosquitto Topic-Level Access Control (read/write restrictions)"]
    end

    subgraph L4["Layer 4: Gateway Sandboxing & Sanitization"]
        WHITELIST["Caller ID Filtering (Whitelist / Blacklist / All)"]
        RATELIMIT["Sliding-Window Rate Limiting (per-minute, per-hour, per-number)"]
        AT_SANDBOX["AT Command Allowlist / Blocklist Protection"]
    end

    subgraph L5["Layer 5: Audit & Monitoring"]
        AUDIT_LOG["Structured Audit Logging (slog with phone masking)"]
        ALERT_TOPIC["Real-Time Security MQTT Alerts (security/rate_limit_hit)"]
    end

    L1 --> L2 --> L3 --> L4 --> L5
```

### Protection Against SMS Bombing and Loops
- **Sliding-Window Rate Limiter**: Tracks global SMS dispatch rates (e.g. 5/min, 30/hour, 100/day) as well as per-recipient limits to eliminate infinite loops in home automation rules.
- **AT Command Sandboxing**: When raw AT command execution via MQTT is enabled, dangerous commands (`AT+CFUN=0`, `AT+CPIN`, `AT&F`, `ATD`) are blocked to prevent modem shutdown, NVRAM corruption, or PIN locking.

---

## 3. Modem Pool & Multi-Modem Routing

When multiple modems are connected, the gateway treats them as a managed pool. The pool supports several routing policies configured via YAML:

```mermaid
flowchart TD
    REQ["Outbound Request\n(SMS or Call)"] --> SELECTOR{"Routing Policy"}

    SELECTOR -->|failover| POL1["1. Try Primary Modem\n2. If busy/down, try Secondary"]
    SELECTOR -->|round_robin| POL2["Even distribution across all ready modems"]
    SELECTOR -->|best_signal| POL3["Route through modem with highest CSQ (dBm)"]
    SELECTOR -->|operator_match| POL4["Match recipient prefix to SIM IMSI/Carrier"]
    SELECTOR -->|sim_redundancy| POL5["Duplicate critical alerts across multiple SIMs"]

    POL1 --> EXEC["Modem Execution"]
    POL2 --> EXEC
    POL3 --> EXEC
    POL4 --> EXEC
    POL5 --> EXEC
```

### Supported Routing Strategies
1. **`failover`** (Default for high availability): Prioritizes the primary modem. If the primary modem loses registration, runs out of balance, or experiences hardware failure, traffic reroutes transparently to the secondary modem.
2. **`round-robin`**: Distributes outbound messages evenly across all operational modems. Useful when modems have separate SMS bundles.
3. **`best-signal`**: Dynamically evaluates the current signal quality (`CSQ`) and directs traffic through the modem experiencing the best connection.
4. **`operator-match`**: Analyzes recipient phone number prefixes and selects the SIM card belonging to the same mobile operator to minimize SMS costs.
5. **`sim-redundancy`**: Sends critical emergency notifications through multiple physical modems simultaneously.

---

## 4. Modem Lifecycle State Machine

Each modem runner operates a state machine that handles hotplugging, automatic reconnection, and error recovery without service restarts:

```mermaid
stateDiagram-v2
    [*] --> Disconnected

    Disconnected --> Connecting: Serial port opened
    Connecting --> Probing: Baud rate synchronized
    Probing --> Initializing: AT echo off (ATE0)
    Initializing --> CheckingSIM: Driver assigned

    CheckingSIM --> Ready: PIN OK, SIM READY, CREG registered
    CheckingSIM --> Degraded: Weak signal or low balance
    CheckingSIM --> Error: SIM missing or locked (CPIN required)

    Ready --> Busy: Executing SMS / Call / USSD
    Busy --> Ready: Operation complete
    Busy --> Recovering: Command timed out / Serial dropped

    Degraded --> Ready: Signal improved / Balance topped up
    Error --> Recovering: Automatic retry timer

    Recovering --> Disconnected: Port reset
    Recovering --> Connecting: Serial re-opened
```

---

## 5. Automated Diagnostic Failure Tree

When an outbound operation fails (e.g. SMS delivery fails with `+CMS ERROR`), GSM2MQTT does not simply return an error; it executes an **automated diagnostic pipeline** to isolate the root cause:

```mermaid
flowchart TD
    FAIL["Operation Failed\n(+CMS ERROR or Timeout)"] --> D1{"Step 1: Check SIM\nAT+CPIN?"}

    D1 -->|Error / Missing| A1["Alert: SIM Card Fault or Locked\n(Publish to /alert and /health)"]
    D1 -->|READY| D2{"Step 2: Check Registration\nAT+CREG?"}

    D2 -->|Searching / Denied| A2["Alert: Network Lost / Roaming Denied"]
    D2 -->|Registered| D3{"Step 3: Check Signal\nAT+CSQ"}

    D3 -->|CSQ < 5 or High BER| A3["Alert: Weak or Jammed GSM Signal"]
    D3 -->|CSQ OK| D4{"Step 4: Check Balance\nUSSD Balance Query"}

    D4 -->|Balance <= 0| A4["Alert: SIM Balance Exhausted"]
    D4 -->|Balance OK| A5["Alert: Upstream SMSC Error / Carrier Congestion"]

    A1 --> PUB["Update Health Status to 'error' or 'degraded'\nPublish Detailed Diagnostics JSON"]
    A2 --> PUB
    A3 --> PUB
    A4 --> PUB
    A5 --> PUB
```

---

## 6. Concurrency and Thread Safety

- **Port Mutexing**: Serial ports are protected by strict per-modem mutexes to prevent interleaving of concurrent AT commands, asynchronous notifications (URCs), and USSD sessions.
- **Context Propagation**: Every operation (SMS, USSD, dial) accepts a `context.Context` with hard deadlines. If a serial line hangs, the timeout triggers, cleans up buffers, and unblocks the calling goroutine.
- **Lock-Free MQTT Publishing**: Alerting and MQTT event dispatch occur outside of internal tariff or driver mutex locks, eliminating deadlock possibilities.
