package api

import (
	"context"

	"github.com/legoser/gsm2mqtt/internal/services"
	"github.com/legoser/gsm2mqtt/internal/tariff"
)

// ServerConfig configures the embedded HTTP Web and REST server.
type ServerConfig struct {
	Host  string
	Port  int
	Token string
}

// ModemSummary is an alias to services.ModemSummary for API presentation.
type ModemSummary = services.ModemSummary

// ReceivedSMS is an alias to services.ReceivedSMS for API presentation.
type ReceivedSMS = services.ReceivedSMS

// CallStatus is an alias to services.CallStatus for API presentation.
type CallStatus = services.CallStatus

// MQTTStatus is an alias to services.MQTTStatus for API presentation.
type MQTTStatus = services.MQTTStatus

// ModemManager is the interface required by the API to query state and dispatch operations.
type ModemManager interface {
	GetModems() []ModemSummary
	SendSMS(ctx context.Context, modemID, to, text string) ([]byte, error)
	SendUSSD(ctx context.Context, modemID, code string) (string, error)
	DialCall(ctx context.Context, modemID, number string) error
	HangupCall(ctx context.Context, modemID string) error
	GetCallStatus(modemID string) CallStatus
	SendRawAT(ctx context.Context, modemID, cmd string) (string, error)
	GetReceivedSMS() []ReceivedSMS
	GetMQTTStatus() MQTTStatus
	UpdateTariffConfig(modemID string, cfg tariff.Config) error
	SetTariffUsage(modemID string, update tariff.UsageUpdate) error
	ResetTariffQuotas(modemID string) error
	GetTariffStatus(modemID string) (*tariff.UsageStatus, error)
}
