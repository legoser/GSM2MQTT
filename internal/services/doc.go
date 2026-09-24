// Package services orchestrates business workflows across modem drivers,
// SMS encoding, security filters, rate limiting, and MQTT messaging.
//
// The package includes:
//
//   - ModemRunner: Manages the lifecycle of an individual GSM modem, opening its
//     serial port, initializing the modem driver, handling incoming URC streams,
//     and wiring MQTT subscriptions and telemetry publishers.
//
//   - GatewayManager: An aggregate facade coordinating multiple active ModemRunners,
//     providing unified REST API and MQTT query access across all connected modems.
//
//   - SMSService: Handles SMS dispatch and reception. Dispatches PDU-encoded messages,
//     enforces rate limits and phone number whitelists/blacklists, and tracks delivery
//     reports. Listens for real-time unsolicited result codes (+CMTI, +CMT, +CDS),
//     automatically retrieves and reassembles incoming multipart SMS, and purges
//     processed messages from modem SIM memory to prevent storage overflows.
//
//   - CallService: Manages voice call operations, supporting outgoing dialing,
//     incoming call detection, DTMF reception, and Call-Drop alert workflows (where
//     an outgoing alert call is automatically terminated immediately upon answer).
//     Maintains a real-time event log timeline (CallLogEntry) for dashboard inspection.
//
//   - StatusService: Regularly polls signal strength (CSQ) and network registration
//     (CREG), caching last known good states during active calls or USSD sessions
//     to prevent transient "degraded" status flips.
//
//   - USSDService: Dispatches USSD requests (e.g. balance queries), decoding inline
//     replies and asynchronous +CUSD indications, and updating the tariff manager.
//
//   - DiagnosticService: Analyzes modem telemetry to detect SIM errors, weak signal,
//     network drops, and exhausted balance, publishing alert events to MQTT.
package services
