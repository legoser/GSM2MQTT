// Package mqtt provides MQTT client abstraction, topic hierarchy generation,
// Home Assistant Auto Discovery payload builders, and message publishing/subscription.
//
// Key components:
//
//   - MQTTClient: Pluggable interface abstracting MQTT connection, subscription,
//     and QoS-aware message publishing.
//
//   - PahoClient: Production client implementation based on Eclipse Paho MQTT.
//     Includes Last Will and Testament (LWT) support, automatic reconnects,
//     and optional TLS/SSL encryption.
//
//   - Topics: Generates standardized topic paths for modem telemetry, incoming
//     SMS, outgoing SMS requests, USSD transactions, voice calls, and diagnostics.
//
//   - Discovery: Builds JSON payloads for Home Assistant MQTT Auto Discovery,
//     allowing Home Assistant to automatically register signal strength sensors,
//     modem health entities, and balance monitors without manual configuration.
package mqtt
