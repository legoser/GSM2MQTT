package services

import (
	"os"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/config"
	"github.com/legoser/gsm2mqtt/internal/sms"
)

func TestModemRunner_InboxPersistenceAndDeduplication(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "inbox_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		Tariff: config.TariffConfig{
			StorageDir: tempDir,
		},
	}
	mCfg := config.ModemConfig{ID: "test_modem"}

	r1 := NewModemRunner(mCfg, cfg, nil, nil)

	msg1 := &sms.AssembledSMS{
		From:      "+79991112233",
		Text:      "Hello world",
		Timestamp: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC),
	}

	// 1. Record first message
	r1.recordIncomingSMS(msg1)
	inbox1 := r1.GetReceivedSMS()
	if len(inbox1) != 1 {
		t.Fatalf("expected 1 message in inbox, got %d", len(inbox1))
	}
	if inbox1[0].Text != "Hello world" {
		t.Errorf("expected text 'Hello world', got %q", inbox1[0].Text)
	}

	// 2. Duplicate message must not be appended
	r1.recordIncomingSMS(msg1)
	inbox2 := r1.GetReceivedSMS()
	if len(inbox2) != 1 {
		t.Fatalf("expected duplicate message to be ignored, got %d messages", len(inbox2))
	}

	// 3. Different message should be appended
	msg2 := &sms.AssembledSMS{
		From:      "+79991112233",
		Text:      "Second message",
		Timestamp: time.Date(2026, 9, 22, 12, 5, 0, 0, time.UTC),
	}
	r1.recordIncomingSMS(msg2)
	inbox3 := r1.GetReceivedSMS()
	if len(inbox3) != 2 {
		t.Fatalf("expected 2 messages in inbox, got %d", len(inbox3))
	}

	// 4. Simulate restart: new ModemRunner with same storage dir must restore all messages
	r2 := NewModemRunner(mCfg, cfg, nil, nil)
	inboxRestored := r2.GetReceivedSMS()
	if len(inboxRestored) != 2 {
		t.Fatalf("expected 2 restored messages after restart, got %d", len(inboxRestored))
	}
	if inboxRestored[0].Text != "Hello world" || inboxRestored[1].Text != "Second message" {
		t.Errorf("unexpected restored message contents: %+v", inboxRestored)
	}

	// 5. Test ClearReceivedSMS
	r2.ClearReceivedSMS()
	if len(r2.GetReceivedSMS()) != 0 {
		t.Fatalf("expected empty inbox after ClearReceivedSMS, got %d", len(r2.GetReceivedSMS()))
	}

	// Verify persistence of cleared inbox
	r3 := NewModemRunner(mCfg, cfg, nil, nil)
	if len(r3.GetReceivedSMS()) != 0 {
		t.Fatalf("expected empty inbox after restart following clear, got %d", len(r3.GetReceivedSMS()))
	}
}
