#!/bin/sh
set -e

if [ "$MOCK_MODEM" = "true" ]; then
    echo "[Entrypoint] MOCK_MODEM=true: Starting virtual GSM modem simulator in background..."
    python3 /app/mock_modem.py &
    sleep 1
fi

# Wait for the modem device if one is configured (container may start before
# udev creates the node). Skipped when no device is set; the app itself
# retries with backoff, so this is bounded and only avoids early log spam.
if [ -n "$MODEM_DEVICE" ] && [ "$MOCK_MODEM" != "true" ]; then
    for i in $(seq 1 30); do
        if [ -e "$MODEM_DEVICE" ]; then
            break
        fi
        echo "[Entrypoint] Waiting for modem device $MODEM_DEVICE ($i/30)..."
        sleep 2
    done
fi

exec /app/gsm2mqtt "$@"
