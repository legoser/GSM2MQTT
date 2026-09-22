FROM golang:alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /build/bin/gsm2mqtt ./cmd/gsm2mqtt/

# Final runtime image
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata curl python3

WORKDIR /app

COPY --from=builder /build/bin/gsm2mqtt /app/gsm2mqtt
COPY configs/gsm2mqtt.yaml /etc/gsm2mqtt/gsm2mqtt.yaml
COPY scripts/mock_modem.py /app/mock_modem.py
COPY deployments/docker/entrypoint.sh /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["--config", "/etc/gsm2mqtt/gsm2mqtt.yaml"]

