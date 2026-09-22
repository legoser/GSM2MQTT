package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/sms"
	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

// SMSServiceConfig configures SMS sending and processing behaviors.
type SMSServiceConfig struct {
	ModemID        string
	Transliterate  bool
	DeliveryReport bool
	DefaultPrefix  string
}

// SendSMSRequest specifies recipient and content for outgoing SMS.
type SendSMSRequest struct {
	To             string `json:"to"`
	Text           string `json:"text"`
	DeliveryReport bool   `json:"delivery_report"`
}

// PDUSender transmits encoded PDU octets to the modem.
type PDUSender interface {
	SendPDU(cmdLength int, pduHex string) (byte, error)
}

// NumberFilter validates whether an external number is allowed.
type NumberFilter interface {
	Check(number string) error
}

// RateLimiter checks rate limits for SMS destinations.
type RateLimiter interface {
	Check(number string) error
}

// SMSService orchestrates SMS filtering, rate limiting, encoding, and delivery tracking.
type SMSService struct {
	cfg        SMSServiceConfig
	sender     PDUSender
	filter     NumberFilter
	limiter    RateLimiter
	tracker    *sms.Tracker
	assembler  *sms.Assembler
	onReceived func(msg *sms.AssembledSMS)
	storageMgr modem.StorageManager
}

// NewSMSService constructs a new SMSService orchestrator.
func NewSMSService(
	cfg SMSServiceConfig,
	sender PDUSender,
	filter NumberFilter,
	limiter RateLimiter,
	tracker *sms.Tracker,
	assembler *sms.Assembler,
	onReceived func(msg *sms.AssembledSMS),
) *SMSService {
	if cfg.DefaultPrefix == "" {
		cfg.DefaultPrefix = "+7"
	}
	return &SMSService{
		cfg:        cfg,
		sender:     sender,
		filter:     filter,
		limiter:    limiter,
		tracker:    tracker,
		assembler:  assembler,
		onReceived: onReceived,
	}
}

// Send validates, encodes, and transmits an SMS message.
func (s *SMSService) Send(ctx context.Context, req SendSMSRequest) ([]byte, error) {
	if s.filter != nil {
		if err := s.filter.Check(req.To); err != nil {
			return nil, err
		}
	}

	if s.limiter != nil {
		if err := s.limiter.Check(req.To); err != nil {
			return nil, err
		}
	}

	normNumber, err := sms.NormalizePhoneNumber(req.To, s.cfg.DefaultPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize phone number: %w", err)
	}

	if err := sms.ValidateMessageText(req.Text); err != nil {
		return nil, fmt.Errorf("invalid SMS text: %w", err)
	}

	text := req.Text
	if s.cfg.Transliterate {
		text = sms.Transliterate(text)
	}

	requestReport := req.DeliveryReport || s.cfg.DeliveryReport
	pdus, err := pdu.EncodeSMS(normNumber, text, pdu.EncodingAuto, requestReport)
	if err != nil {
		return nil, fmt.Errorf("failed to encode SMS PDU: %w", err)
	}

	return s.dispatchPDUs(pdus, normNumber, text, requestReport)
}

// HandleURC processes incoming SMS indications (+CDS, +CMT).
func (s *SMSService) HandleURC(urc string) {
	trimmed := strings.TrimSpace(urc)

	switch {
	case strings.HasPrefix(trimmed, "+CDS:"):
		s.handleDeliveryReportURC(trimmed)
	case strings.HasPrefix(trimmed, "+CMT:"):
		s.handleIncomingSMSURC(trimmed)
	}
}

func (s *SMSService) dispatchPDUs(pdus []pdu.PDU, normNumber, text string, requestReport bool) ([]byte, error) {
	refs := make([]byte, len(pdus))
	for i, part := range pdus {
		ref, err := s.sender.SendPDU(part.CommandLength, part.Hex)
		if err != nil {
			slog.Error("failed to send PDU part",
				slog.String("modem", s.cfg.ModemID),
				slog.Int("part", i+1),
				slog.Int("total", len(pdus)),
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("failed to send PDU part %d/%d: %w", i+1, len(pdus), err)
		}
		refs[i] = ref

		if requestReport && s.tracker != nil {
			s.tracker.Track(ref, normNumber, text, s.cfg.ModemID)
		}
	}
	return refs, nil
}

func (s *SMSService) handleDeliveryReportURC(urc string) {
	if s.tracker == nil {
		return
	}
	pduHex := extractLastHexToken(urc)
	if pduHex == "" {
		return
	}
	report, err := pdu.DecodeStatusReport(pduHex)
	if err == nil {
		s.tracker.HandleReport(report)
	}
}

func (s *SMSService) handleIncomingSMSURC(urc string) {
	if s.assembler == nil {
		return
	}
	pduHex := extractLastHexToken(urc)
	if pduHex == "" {
		return
	}
	decoded, err := pdu.DecodeSMS(pduHex)
	if err != nil {
		return
	}

	part := sms.IncomingPart{
		From:        decoded.From,
		Text:        decoded.Text,
		Timestamp:   decoded.Timestamp,
		IsMultipart: decoded.HasUDH,
		Reference:   decoded.Reference,
		PartNumber:  decoded.PartNumber,
		TotalParts:  decoded.TotalParts,
		Encoding:    string(decoded.Encoding),
	}

	assembled, complete := s.assembler.AddPart(part)
	if complete && s.onReceived != nil {
		slog.Info("incoming SMS assembled", slog.String("modem", s.cfg.ModemID), slog.String("from", assembled.From), slog.Int("segments", assembled.Segments))
		s.onReceived(assembled)
	}
}

func extractLastHexToken(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
