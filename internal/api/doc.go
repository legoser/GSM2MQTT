// Package api provides a lightweight embedded HTTP REST and Web interface
// built strictly with the Go standard library for modem health inspection,
// diagnostic querying, SMS messaging, voice call control, and direct AT debugging.
//
// Endpoints provided:
//
//   - GET  /api/modems: Returns an array of registered modems, health status, and balance.
//   - GET  /api/mqtt/status: Returns connection state and configuration of the MQTT broker.
//   - POST /api/sms/send: Dispatches an SMS message via the specified or default modem.
//   - GET  /api/sms/inbox: Lists recently received and assembled SMS messages.
//   - POST /api/ussd/send: Executes a USSD query (e.g. *100#) and returns the reply.
//   - POST /api/call/dial: Initiates an outgoing voice call (supporting auto call-drop).
//   - POST /api/call/hangup: Terminates any active voice call.
//   - GET  /api/call/status: Returns real-time call state and event timeline logs.
//   - POST /api/at/send: Executes raw AT commands for direct modem diagnostics.
//   - GET  /health: Health check endpoint returning {"status":"ok"}.
//   - GET  /metrics: Prometheus metrics endpoint.
//   - GET  /favicon.ico: Pager emoji SVG favicon.
//   - GET  /: Web Dashboard single-page application.
package api
