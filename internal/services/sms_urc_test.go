package services

import (
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem"
	"github.com/legoser/gsm2mqtt/internal/security"
	"github.com/legoser/gsm2mqtt/internal/sms"
)

type mockSyncStorageManager struct {
	mu             sync.Mutex
	selectedArea   string
	messages       map[string][]modem.StoredMessage
	deletedIndices []int
	syncCalled     chan string
}

func newMockSyncStorageManager() *mockSyncStorageManager {
	return &mockSyncStorageManager{
		messages:   make(map[string][]modem.StoredMessage),
		syncCalled: make(chan string, 10),
	}
}

func (m *mockSyncStorageManager) SelectStorage(mem string) (*modem.StorageStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selectedArea = mem
	msgs := m.messages[mem]
	return &modem.StorageStatus{Name: mem, Used: len(msgs), Total: 15}, nil
}

func (m *mockSyncStorageManager) ListMessages() ([]modem.StoredMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncCalled <- m.selectedArea
	return m.messages[m.selectedArea], nil
}

func (m *mockSyncStorageManager) DeleteMessage(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedIndices = append(m.deletedIndices, index)
	return nil
}

func (m *mockSyncStorageManager) StorageCapacity() (*modem.StorageStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &modem.StorageStatus{Name: "SM", Used: len(m.messages["SM"]), Total: 15}, nil
}

func TestSMSService_HandleURC_CMTI_SyncsAndPurges(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	var mu sync.Mutex
	var received []*sms.AssembledSMS
	receivedCh := make(chan *sms.AssembledSMS, 5)
	onReceived := func(msg *sms.AssembledSMS) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
		receivedCh <- msg
	}

	svc := NewSMSService(SMSServiceConfig{ModemID: "neoway_m590"}, sender, filter, limiter, tracker, assembler, onReceived)

	storage := newMockSyncStorageManager()
	storage.messages["SM"] = []modem.StoredMessage{
		{
			Index:  1,
			Status: 0,
			// Valid SMS PDU: from +79011111111, text "Test"
			PDUHex: "07919720131111F1040C919701111111F100006290221153252104D4F29C0E",
		},
	}
	svc.SetStorageManager(storage)

	// Simulate incoming CMTI notification from modem
	svc.HandleURC(`+CMTI: "SM", 1`)

	select {
	case msg := <-receivedCh:
		if msg.Text != "Test" {
			t.Errorf("expected text 'Test', got %q", msg.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for CMTI sync and SMS arrival")
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()
	if len(storage.deletedIndices) != 1 || storage.deletedIndices[0] != 1 {
		t.Errorf("expected index 1 to be deleted from storage, got %v", storage.deletedIndices)
	}
}

func TestSMSService_HandleURC_CMTI_CustomStorage(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	receivedCh := make(chan *sms.AssembledSMS, 5)
	onReceived := func(msg *sms.AssembledSMS) {
		receivedCh <- msg
	}

	svc := NewSMSService(SMSServiceConfig{ModemID: "neoway_m590"}, sender, filter, limiter, tracker, assembler, onReceived)
	storage := newMockSyncStorageManager()
	storage.messages["ME"] = []modem.StoredMessage{
		{
			Index:  5,
			Status: 0,
			PDUHex: "07919720131111F1040C919701111111F100006290221153252104D4F29C0E",
		},
	}
	svc.SetStorageManager(storage)

	svc.HandleURC(`+CMTI: "ME",5`)

	select {
	case <-receivedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ME storage CMTI sync")
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.selectedArea != "ME" {
		t.Errorf("expected storage ME, got %q", storage.selectedArea)
	}
	if len(storage.deletedIndices) != 1 || storage.deletedIndices[0] != 5 {
		t.Errorf("expected index 5 deleted, got %v", storage.deletedIndices)
	}
}

func TestSMSService_HandleURC_CMT_TwoLines(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	receivedCh := make(chan *sms.AssembledSMS, 5)
	onReceived := func(msg *sms.AssembledSMS) {
		receivedCh <- msg
	}

	svc := NewSMSService(SMSServiceConfig{ModemID: "generic"}, sender, filter, limiter, tracker, assembler, onReceived)

	// Line 1: Header with length
	svc.HandleURC("+CMT: ,24")
	// Line 2: PDU payload
	svc.HandleURC("07919720131111F1040C919701111111F100006290221153252104D4F29C0E")

	select {
	case msg := <-receivedCh:
		if msg.Text != "Test" {
			t.Errorf("expected text 'Test', got %q", msg.Text)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for two-line CMT SMS dispatch")
	}
}

func TestSMSService_HandleURC_Negative(t *testing.T) {
	sender := &mockPDUSender{}
	filter := security.NewFilter("all", nil, nil)
	limiter := security.NewRateLimiter(10)
	tracker := sms.NewTracker(time.Minute, nil)
	assembler := sms.NewAssembler(time.Hour)

	receivedCh := make(chan *sms.AssembledSMS, 5)
	onReceived := func(msg *sms.AssembledSMS) {
		receivedCh <- msg
	}

	svc := NewSMSService(SMSServiceConfig{ModemID: "generic"}, sender, filter, limiter, tracker, assembler, onReceived)

	t.Run("empty and whitespace lines", func(t *testing.T) {
		svc.HandleURC("")
		svc.HandleURC("   ")
		svc.HandleURC("\r\n")
		select {
		case <-receivedCh:
			t.Fatal("unexpected message received on empty URC")
		case <-time.After(10 * time.Millisecond):
		}
	})

	t.Run("corrupt CMT two-line with non-hex payload", func(t *testing.T) {
		svc.HandleURC("+CMT: ,24")
		svc.HandleURC("CORRUPT_PAYLOAD_NOT_HEX_00!!")
		select {
		case <-receivedCh:
			t.Fatal("unexpected message received on corrupt PDU")
		case <-time.After(10 * time.Millisecond):
		}
	})

	t.Run("CMTI fallback on malformed URC", func(t *testing.T) {
		mem := parseCMTIStorage("+CMTI:")
		if mem != "SM" {
			t.Errorf("expected default SM on malformed CMTI, got %q", mem)
		}
	})
}
