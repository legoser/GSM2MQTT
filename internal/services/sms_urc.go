package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/sms"
	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

// HandleURC processes incoming SMS notifications (+CMTI, +CMT, +CDS).
func (s *SMSService) HandleURC(urc string) {
	trimmed := strings.TrimSpace(urc)

	switch {
	case strings.HasPrefix(trimmed, "+CMTI:"):
		s.handleCMTIURC(trimmed)
	case strings.HasPrefix(trimmed, "+CMT:"):
		s.handleIncomingSMSURC(trimmed)
	case strings.HasPrefix(trimmed, "+CDS:"):
		s.handleDeliveryReportURC(trimmed)
	default:
		s.handleContinuedURC(trimmed)
	}
}

func (s *SMSService) handleContinuedURC(trimmed string) {
	s.urcMu.Lock()
	expectingCMT := s.expectingCMT
	expectingCDS := s.expectingCDS
	s.expectingCMT = false
	s.expectingCDS = false
	s.urcMu.Unlock()

	if expectingCMT && isHexPDU(trimmed) {
		s.decodeAndDispatchPDU(trimmed)
	} else if expectingCDS && isHexPDU(trimmed) {
		s.decodeAndDispatchReport(trimmed)
	}
}

// parseCMTIStorage extracts storage name from +CMTI: "<mem>", <index>
func parseCMTIStorage(urc string) string {
	payload := strings.TrimPrefix(urc, "+CMTI:")
	payload = strings.TrimSpace(payload)
	parts := strings.Split(payload, ",")
	if len(parts) > 0 {
		mem := strings.Trim(strings.TrimSpace(parts[0]), "\" ")
		if mem != "" {
			return mem
		}
	}
	return "SM"
}

func (s *SMSService) handleCMTIURC(urc string) {
	mem := parseCMTIStorage(urc)
	slog.Info("incoming SMS notification (+CMTI)",
		slog.String("modem", s.cfg.ModemID),
		slog.String("storage", mem),
		slog.String("urc", urc),
	)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		storages := []string{mem}
		if mem == "MT" || mem == "" {
			storages = []string{"SM", "ME"}
		}
		count, err := s.SyncStoredMessages(ctx, storages...)
		if err != nil {
			slog.Warn("failed to sync stored messages on CMTI URC",
				slog.String("modem", s.cfg.ModemID),
				slog.String("storage", mem),
				slog.Any("error", err),
			)
			return
		}
		slog.Info("synced and purged stored SMS messages on CMTI",
			slog.String("modem", s.cfg.ModemID),
			slog.String("storage", mem),
			slog.Int("count", count),
		)
	}()
}

func (s *SMSService) handleIncomingSMSURC(urc string) {
	pduHex := extractLastHexToken(urc)
	if pduHex != "" && isHexPDU(pduHex) {
		s.decodeAndDispatchPDU(pduHex)
		return
	}
	s.urcMu.Lock()
	s.expectingCMT = true
	s.urcMu.Unlock()
	slog.Debug("expecting CMT PDU payload on next URC line", slog.String("modem", s.cfg.ModemID))
}

func (s *SMSService) decodeAndDispatchPDU(pduHex string) {
	if s.assembler == nil {
		return
	}
	slog.Debug("decoding incoming SMS PDU", slog.String("modem", s.cfg.ModemID), slog.Int("len", len(pduHex)))
	decoded, err := pdu.DecodeSMS(pduHex)
	if err != nil {
		slog.Warn("failed to decode incoming SMS PDU", slog.String("modem", s.cfg.ModemID), slog.Any("error", err))
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

func (s *SMSService) handleDeliveryReportURC(urc string) {
	pduHex := extractLastHexToken(urc)
	if pduHex != "" && isHexPDU(pduHex) {
		s.decodeAndDispatchReport(pduHex)
		return
	}
	s.urcMu.Lock()
	s.expectingCDS = true
	s.urcMu.Unlock()
	slog.Debug("expecting CDS delivery report PDU on next URC line", slog.String("modem", s.cfg.ModemID))
}

func (s *SMSService) decodeAndDispatchReport(pduHex string) {
	if s.tracker == nil {
		return
	}
	report, err := pdu.DecodeStatusReport(pduHex)
	if err != nil {
		slog.Warn("failed to decode delivery report PDU", slog.String("modem", s.cfg.ModemID), slog.Any("error", err))
		return
	}
	s.tracker.HandleReport(report)
}

func isHexPDU(s string) bool {
	if len(s) < 10 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func extractLastHexToken(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
