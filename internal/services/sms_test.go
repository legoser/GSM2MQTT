package services

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/sms"
)

type mockPDUSender struct {
	mu       sync.Mutex
	sentPDUs []string
	lengths  []int
	nextRef  byte
	err      error
}

func (m *mockPDUSender) SendPDU(ctx context.Context, cmdLength int, pduHex string) (byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	m.lengths = append(m.lengths, cmdLength)
	m.sentPDUs = append(m.sentPDUs, pduHex)
	m.nextRef++
	return m.nextRef, nil
}

func TestSMSService_Send_Success(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	cfg := SMSServiceConfig{
		ModemID:        "siemens_tc35",
		Transliterate:  false,
		DeliveryReport: true,
	}

	svc := NewSMSService(cfg, sender, filter, limiter, tracker, assembler, nil)

	ctx := context.Background()
	req := SendSMSRequest{
		To:             "+79991112233",
		Text:           "Test message",
		DeliveryReport: true,
	}

	refs, err := svc.Send(ctx, req)
	if err != nil {
		t.Fatalf("unexpected Send error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 reference, got %v", refs)
	}
	if len(sender.sentPDUs) != 1 {
		t.Fatalf("expected 1 PDU sent, got %d", len(sender.sentPDUs))
	}
}

func TestSMSService_Send_FilterBlocked(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("blacklist", nil, []string{"+79998887766"})
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	svc := NewSMSService(SMSServiceConfig{ModemID: "siemens_tc35"}, sender, filter, limiter, tracker, assembler, nil)

	ctx := context.Background()
	req := SendSMSRequest{
		To:   "+79998887766",
		Text: "Blocked text",
	}

	_, err := svc.Send(ctx, req)
	if !errors.Is(err, security.ErrNumberBlocked) {
		t.Fatalf("expected ErrNumberBlocked, got: %v", err)
	}
	if len(sender.sentPDUs) != 0 {
		t.Errorf("no PDUs should be sent when filtered")
	}
}

func TestSMSService_Send_RateLimitExceeded(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(1)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	svc := NewSMSService(SMSServiceConfig{ModemID: "siemens_tc35"}, sender, filter, limiter, tracker, assembler, nil)

	ctx := context.Background()
	req := SendSMSRequest{
		To:   "+79991112233",
		Text: "First message",
	}

	// 1st passes
	if _, err := svc.Send(ctx, req); err != nil {
		t.Fatalf("attempt 1 error: %v", err)
	}

	// 2nd fails rate limit
	_, err := svc.Send(ctx, req)
	if !errors.Is(err, security.ErrRateLimitMinuteExceeded) {
		t.Fatalf("expected ErrRateLimitMinuteExceeded, got: %v", err)
	}
}

func TestSMSService_Send_Transliteration(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	cfg := SMSServiceConfig{
		ModemID:       "siemens_tc35",
		Transliterate: true,
	}

	svc := NewSMSService(cfg, sender, filter, limiter, tracker, assembler, nil)

	ctx := context.Background()
	req := SendSMSRequest{
		To:   "+79991112233",
		Text: "Привет мир", // Cyrillic
	}

	_, err := svc.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if len(sender.sentPDUs) != 1 {
		t.Fatalf("expected 1 PDU sent")
	}
}

func TestSMSService_Send_Multipart(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	svc := NewSMSService(SMSServiceConfig{ModemID: "siemens_tc35"}, sender, filter, limiter, tracker, assembler, nil)

	ctx := context.Background()
	// Create text with 200 GSM-7 chars (exceeds single 160 limit)
	longText := strings.Repeat("A", 200)

	req := SendSMSRequest{
		To:   "+79991112233",
		Text: longText,
	}

	refs, err := svc.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("expected 2 parts for 200 runes, got %d", len(refs))
	}
	if len(sender.sentPDUs) != 2 {
		t.Errorf("expected 2 sent PDUs, got %d", len(sender.sentPDUs))
	}
}

type mockStorageManager struct {
	selectedStorage string
	messages        map[string][]modem.StoredMessage
	deletedIndices  []int
}

func (m *mockStorageManager) SelectStorage(mem string) (*modem.StorageStatus, error) {
	m.selectedStorage = mem
	msgs := m.messages[mem]
	return &modem.StorageStatus{Name: mem, Used: len(msgs), Total: 25}, nil
}

func (m *mockStorageManager) ListMessages() ([]modem.StoredMessage, error) {
	return m.messages[m.selectedStorage], nil
}

func (m *mockStorageManager) DeleteMessage(index int) error {
	m.deletedIndices = append(m.deletedIndices, index)
	return nil
}

func (m *mockStorageManager) StorageCapacity() (*modem.StorageStatus, error) {
	return &modem.StorageStatus{Name: "SM", Used: 15, Total: 15}, nil
}

func TestSMSService_SyncStoredMessages(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	var received []*sms.AssembledSMS
	onReceived := func(msg *sms.AssembledSMS) {
		received = append(received, msg)
	}

	svc := NewSMSService(SMSServiceConfig{ModemID: "huawei_e1550"}, sender, filter, limiter, tracker, assembler, onReceived)

	storage := &mockStorageManager{
		messages: map[string][]modem.StoredMessage{
			"SM": {
				{
					Index:  0,
					Status: 0,
					// Valid SMS PDU: from +79011111111, text "Test"
					PDUHex: "07919720131111F1040C919701111111F100006290221153252104D4F29C0E",
				},
			},
		},
	}
	svc.SetStorageManager(storage)

	ctx := context.Background()
	count, err := svc.SyncStoredMessages(ctx, "SM")
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 message synced, got %d", count)
	}
	if len(received) != 1 {
		t.Fatalf("expected 1 received message dispatched, got %d", len(received))
	}
	if received[0].Text != "Test" {
		t.Errorf("expected 'Test', got %q", received[0].Text)
	}
	if len(storage.deletedIndices) != 1 || storage.deletedIndices[0] != 0 {
		t.Errorf("expected message 0 deleted, got %v", storage.deletedIndices)
	}
}
