package services

import (
	"os"
	"testing"

	"github.com/legoser/gsm2mqtt/internal/config"
)

func TestModemRunner_CallHistoryPersistenceAndClear(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "call_history_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		Tariff: config.TariffConfig{
			StorageDir: tempDir,
		},
	}
	mCfg := config.ModemConfig{ID: "modem_calls"}

	r1 := NewModemRunner(mCfg, cfg, nil, nil)

	// 1. Initial history is empty
	if len(r1.GetCallHistory()) != 0 {
		t.Fatalf("expected empty initial call history, got %d", len(r1.GetCallHistory()))
	}

	// 2. Record incoming missed call
	r1.recordCall(CallRecord{
		Number:    "+79991234567",
		Direction: "incoming",
		Status:    "missed",
		Duration:  0,
	})

	// 3. Record outgoing completed call
	r1.recordCall(CallRecord{
		Number:    "+79110000000",
		Direction: "outgoing",
		Status:    "completed",
		Duration:  45,
	})

	calls := r1.GetCallHistory()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls in history, got %d", len(calls))
	}
	if calls[0].Status != "missed" || calls[0].Number != "+79991234567" {
		t.Errorf("unexpected first call: %+v", calls[0])
	}
	if calls[1].Status != "completed" || calls[1].Duration != 45 {
		t.Errorf("unexpected second call: %+v", calls[1])
	}

	// 4. Test persistence across runner restart
	r2 := NewModemRunner(mCfg, cfg, nil, nil)
	restoredCalls := r2.GetCallHistory()
	if len(restoredCalls) != 2 {
		t.Fatalf("expected 2 restored calls after restart, got %d", len(restoredCalls))
	}
	if restoredCalls[0].Number != "+79991234567" || restoredCalls[1].Number != "+79110000000" {
		t.Errorf("unexpected restored calls content: %+v", restoredCalls)
	}

	// 5. Test ClearCallHistory
	r2.ClearCallHistory()
	if len(r2.GetCallHistory()) != 0 {
		t.Fatalf("expected empty call history after ClearCallHistory, got %d", len(r2.GetCallHistory()))
	}

	// Verify persistence of cleared state
	r3 := NewModemRunner(mCfg, cfg, nil, nil)
	if len(r3.GetCallHistory()) != 0 {
		t.Fatalf("expected empty call history after restart following clear, got %d", len(r3.GetCallHistory()))
	}
}
