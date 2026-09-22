#!/bin/sh
set -e

if [ "$MOCK_MODEM" = "true" ]; then
    echo "[Entrypoint] MOCK_MODEM=true: Starting virtual GSM modem simulator in background..."
    python3 /app/mock_modem.py &
    sleep 1
fi

exec /app/gsm2mqtt "$@"
