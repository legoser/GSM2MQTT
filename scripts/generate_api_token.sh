#!/bin/bash
# Generates a random 32-character hex API token for GSM2MQTT

TOKEN=$(openssl rand -hex 16)
echo "Generated GSM2MQTT_API_TOKEN:"
echo "$TOKEN"
echo ""
echo "Please add the following line to your .env file:"
echo "GSM2MQTT_API_TOKEN=$TOKEN"
