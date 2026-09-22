package tariff

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileStore_SaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tariff_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewFileStore(tempDir)
	modemID := "huawei_e1550"

	now := time.Now().UTC().Truncate(time.Second)
	state := State{
		Balance:               -1.78,
		Currency:              DefaultCurrency,
		SMSDayCount:           3,
		SMSMonthCount:         15,
		CallMinutesUsed:       12.5,
		DataBytesUsed:         1048576,
		LastBalanceCheck:      now,
		LastDailyResetDate:    "2026-09-22",
		LastMonthlyResetMonth: "2026-09",
	}

	if err := store.Save(modemID, state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := store.Load(modemID)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state")
	}

	if loaded.Balance != state.Balance {
		t.Errorf("Balance = %v, want %v", loaded.Balance, state.Balance)
	}
	if loaded.Currency != state.Currency {
		t.Errorf("Currency = %v, want %v", loaded.Currency, state.Currency)
	}
	if loaded.SMSDayCount != state.SMSDayCount {
		t.Errorf("SMSDayCount = %v, want %v", loaded.SMSDayCount, state.SMSDayCount)
	}
	if loaded.SMSMonthCount != state.SMSMonthCount {
		t.Errorf("SMSMonthCount = %v, want %v", loaded.SMSMonthCount, state.SMSMonthCount)
	}
	if loaded.CallMinutesUsed != state.CallMinutesUsed {
		t.Errorf("CallMinutesUsed = %v, want %v", loaded.CallMinutesUsed, state.CallMinutesUsed)
	}
	if loaded.DataBytesUsed != state.DataBytesUsed {
		t.Errorf("DataBytesUsed = %v, want %v", loaded.DataBytesUsed, state.DataBytesUsed)
	}
	if loaded.LastDailyResetDate != state.LastDailyResetDate {
		t.Errorf("LastDailyResetDate = %v, want %v", loaded.LastDailyResetDate, state.LastDailyResetDate)
	}
	if loaded.LastMonthlyResetMonth != state.LastMonthlyResetMonth {
		t.Errorf("LastMonthlyResetMonth = %v, want %v", loaded.LastMonthlyResetMonth, state.LastMonthlyResetMonth)
	}
}

func TestFileStore_NonExistentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tariff_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewFileStore(tempDir)
	loaded, err := store.Load("unknown_modem")
	if err != nil {
		t.Fatalf("unexpected error on missing file: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil state for non-existent modem, got %+v", loaded)
	}
}

func TestFileStore_CorruptedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tariff_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "tariff_bad_modem.json")
	if err := os.WriteFile(filePath, []byte("NOT_VALID_JSON{{{"), 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	store := NewFileStore(tempDir)
	_, err = store.Load("bad_modem")
	if err == nil {
		t.Fatal("expected error on corrupted json, got nil")
	}
}

func TestManager_PersistenceAcrossRestarts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tariff_manager_persist_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := Config{
		Enabled:          true,
		SMSLimit:         100,
		CallMinutesLimit: 50.0,
		ResetDayOfMonth:  1,
		StorageDir:       tempDir,
	}

	store := NewFileStore(tempDir)
	m1 := NewManager("huawei_e1550", cfg, nil)
	m1.SetStore(store)

	m1.RecordSMS(12)
	m1.RecordCallMinutes(5.5)
	m1.RecordData(2048)
	m1.UpdateBalance(-1.78, DefaultCurrency)

	// Create a second manager instance mimicking application restart
	m2 := NewManager("huawei_e1550", cfg, nil)
	m2.SetStore(store)

	st := m2.Status()
	if st.SMSMonthCount != 12 {
		t.Errorf("expected restored SMSMonthCount = 12, got %d", st.SMSMonthCount)
	}
	if st.SMSDayCount != 12 {
		t.Errorf("expected restored SMSDayCount = 12, got %d", st.SMSDayCount)
	}
	if st.CallMinutesUsed != 5.5 {
		t.Errorf("expected restored CallMinutesUsed = 5.5, got %v", st.CallMinutesUsed)
	}
	if st.DataBytesUsed != 2048 {
		t.Errorf("expected restored DataBytesUsed = 2048, got %d", st.DataBytesUsed)
	}
	if st.Balance != -1.78 {
		t.Errorf("expected restored Balance = -1.78, got %v", st.Balance)
	}
}

func TestManager_Alerts_CallMinutesQuotas(t *testing.T) {
	var alerts []AlertEvent
	var mu sync.Mutex

	cfg := Config{
		Enabled:          true,
		CallMinutesLimit: 100.0,
	}

	m := NewManager("huawei_e1550", cfg, func(a AlertEvent) {
		mu.Lock()
		defer mu.Unlock()
		alerts = append(alerts, a)
	})

	m.RecordCallMinutes(89.0)
	if len(alerts) != 0 {
		t.Errorf("expected no alert at 89 mins, got %d", len(alerts))
	}

	m.RecordCallMinutes(1.0) // 90 mins -> 90%
	mu.Lock()
	if len(alerts) != 1 || alerts[0].Type != "call_limit_warning" {
		t.Errorf("expected call_limit_warning, got %+v", alerts)
	}
	mu.Unlock()

	m.RecordCallMinutes(10.0) // 100 mins -> 100%
	mu.Lock()
	if len(alerts) != 2 || alerts[1].Type != "call_limit_exceeded" {
		t.Errorf("expected call_limit_exceeded, got %+v", alerts)
	}
	mu.Unlock()
}

func TestManager_Alerts_DataTrafficQuotas(t *testing.T) {
	var alerts []AlertEvent
	var mu sync.Mutex

	cfg := Config{
		Enabled:            true,
		DataTrafficLimitMB: 100, // 100 MB
	}

	m := NewManager("huawei_e1550", cfg, func(a AlertEvent) {
		mu.Lock()
		defer mu.Unlock()
		alerts = append(alerts, a)
	})

	const mb = 1024 * 1024
	m.RecordData(89 * mb)
	if len(alerts) != 0 {
		t.Errorf("expected no alert at 89 MB, got %d", len(alerts))
	}

	m.RecordData(1 * mb) // 90 MB -> 90%
	mu.Lock()
	if len(alerts) != 1 || alerts[0].Type != "data_limit_warning" {
		t.Errorf("expected data_limit_warning, got %+v", alerts)
	}
	mu.Unlock()

	m.RecordData(10 * mb) // 100 MB -> 100%
	mu.Lock()
	if len(alerts) != 2 || alerts[1].Type != "data_limit_exceeded" {
		t.Errorf("expected data_limit_exceeded, got %+v", alerts)
	}
	mu.Unlock()
}
