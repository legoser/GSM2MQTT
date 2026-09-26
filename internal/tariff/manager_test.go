package tariff

import (
	"sync"
	"testing"
)

func TestManager_RecordSMS(t *testing.T) {
	cfg := Config{
		Enabled:         true,
		SMSLimit:        100,
		ResetDayOfMonth: 1,
	}

	m := NewManager("siemens_tc35", cfg, nil)
	m.RecordSMS(5)

	status := m.Status()
	if status.SMSDayCount != 5 {
		t.Errorf("expected SMSDayCount = 5, got %d", status.SMSDayCount)
	}
	if status.SMSMonthCount != 5 {
		t.Errorf("expected SMSMonthCount = 5, got %d", status.SMSMonthCount)
	}
	if status.SMSRemaining != 95 {
		t.Errorf("expected SMSRemaining = 95, got %d", status.SMSRemaining)
	}
}

func TestManager_Alerts_QuotaThresholds(t *testing.T) {
	var alerts []AlertEvent
	var mu sync.Mutex

	cfg := Config{
		Enabled:         true,
		SMSLimit:        100,
		MinBalanceAlert: 50.0,
	}

	m := NewManager("siemens_tc35", cfg, func(a AlertEvent) {
		mu.Lock()
		defer mu.Unlock()
		alerts = append(alerts, a)
	})

	// 1. Send 89 messages -> no warning yet
	m.RecordSMS(89)
	if len(alerts) != 0 {
		t.Errorf("expected no alert at 89 SMS, got %d", len(alerts))
	}

	// 2. Reach 90% threshold -> warning alert
	m.RecordSMS(1) // total 90
	mu.Lock()
	if len(alerts) != 1 || alerts[0].Type != "sms_limit_warning" {
		t.Errorf("expected sms_limit_warning alert, got %+v", alerts)
	}
	mu.Unlock()

	// 3. Reach 100% limit -> exceeded alert
	m.RecordSMS(10) // total 100
	mu.Lock()
	if len(alerts) != 2 || alerts[1].Type != "sms_limit_exceeded" {
		t.Errorf("expected sms_limit_exceeded alert, got %+v", alerts)
	}
	mu.Unlock()
}

func TestManager_Alert_LowBalance(t *testing.T) {
	var alerts []AlertEvent
	var mu sync.Mutex

	cfg := Config{
		Enabled:         true,
		MinBalanceAlert: 50.0,
	}

	m := NewManager("siemens_tc35", cfg, func(a AlertEvent) {
		mu.Lock()
		defer mu.Unlock()
		alerts = append(alerts, a)
	})

	// Initial normal balance
	m.UpdateBalance(150.0, "RUB")
	if len(alerts) != 0 {
		t.Errorf("expected no alert for normal balance, got %d", len(alerts))
	}

	// Drop below minimum threshold
	m.UpdateBalance(35.50, "RUB")
	if !m.Status().LowBalance {
		t.Errorf("expected Status().LowBalance = true when balance is 35.50 < 50.0")
	}
	mu.Lock()
	if len(alerts) != 1 || alerts[0].Type != "low_balance" {
		t.Fatalf("expected low_balance alert, got %+v", alerts)
	}
	if alerts[0].Value != 35.50 {
		t.Errorf("expected alert value 35.50, got %v", alerts[0].Value)
	}
	mu.Unlock()

	// Second check below threshold shouldn't spam duplicate alert
	m.UpdateBalance(30.00, "RUB")
	if !m.Status().LowBalance {
		t.Errorf("expected Status().LowBalance = true when balance is 30.00 < 50.0")
	}
	mu.Lock()
	if len(alerts) != 1 {
		t.Errorf("expected duplicate alert to be suppressed, got %d alerts", len(alerts))
	}
	mu.Unlock()

	// Recover above threshold
	m.UpdateBalance(100.0, "RUB")
	if m.Status().LowBalance {
		t.Errorf("expected Status().LowBalance = false when balance is 100.0 >= 50.0")
	}

	// Drop again -> new alert
	m.UpdateBalance(20.0, "RUB")
	mu.Lock()
	if len(alerts) != 2 {
		t.Errorf("expected new alert after recovery and drop, got %d alerts", len(alerts))
	}
	mu.Unlock()
}

func TestManager_ResetCounters(t *testing.T) {
	cfg := Config{
		Enabled:         true,
		SMSLimit:        100,
		ResetDayOfMonth: 15,
	}

	m := NewManager("siemens_tc35", cfg, nil)
	m.RecordSMS(10)
	m.RecordData(1024)

	// Simulate day change
	m.ResetDaily()
	st := m.Status()
	if st.SMSDayCount != 0 {
		t.Errorf("expected SMSDayCount = 0 after daily reset, got %d", st.SMSDayCount)
	}
	if st.SMSMonthCount != 10 {
		t.Errorf("expected SMSMonthCount = 10, got %d", st.SMSMonthCount)
	}
	if st.DataBytesUsed != 1024 {
		t.Errorf("expected DataBytesUsed = 1024, got %d", st.DataBytesUsed)
	}

	// Simulate billing cycle reset
	m.ResetMonthly()
	st = m.Status()
	if st.SMSMonthCount != 0 {
		t.Errorf("expected SMSMonthCount = 0 after monthly reset, got %d", st.SMSMonthCount)
	}
}

func TestManager_UpdateConfigAndResetQuotas(t *testing.T) {
	tempDir := t.TempDir()
	store := NewFileStore(tempDir)

	initialCfg := Config{
		Enabled:         true,
		SMSLimit:        100,
		ResetDayOfMonth: 1,
	}

	m := NewManager("modem_test", initialCfg, nil)
	m.SetStore(store)
	m.RecordSMS(15)

	// Update configuration dynamically
	m.UpdateConfig(Config{
		SMSLimit:         300,
		CallMinutesLimit: 60,
		ResetDayOfMonth:  15,
		MinBalanceAlert:  40.0,
		BalanceUSSD:      "*105#",
	})

	cfg := m.GetConfig()
	if cfg.SMSLimit != 300 || cfg.CallMinutesLimit != 60 || cfg.ResetDayOfMonth != 15 || cfg.MinBalanceAlert != 40.0 || cfg.BalanceUSSD != "*105#" {
		t.Errorf("unexpected updated config: %+v", cfg)
	}

	status := m.Status()
	if status.SMSLimit != 300 {
		t.Errorf("expected SMSLimit = 300, got %d", status.SMSLimit)
	}
	if status.SMSRemaining != 285 {
		t.Errorf("expected SMSRemaining = 285, got %d", status.SMSRemaining)
	}

	// Reset quotas
	m.ResetQuotas()
	status = m.Status()
	if status.SMSMonthCount != 0 {
		t.Errorf("expected SMSMonthCount = 0 after ResetQuotas, got %d", status.SMSMonthCount)
	}
	if status.SMSRemaining != 300 {
		t.Errorf("expected SMSRemaining = 300 after ResetQuotas, got %d", status.SMSRemaining)
	}

	// Create new manager instance: verify that newly supplied config (e.g. SMSLimit: 100, MinBalanceAlert: 5.0)
	// takes precedence over stored limits from previous runs, while usage state is preserved.
	m2 := NewManager("modem_test", Config{SMSLimit: 100, MinBalanceAlert: 5.0}, nil)
	m2.SetStore(store)
	cfg2 := m2.GetConfig()
	if cfg2.SMSLimit != 100 || cfg2.MinBalanceAlert != 5.0 {
		t.Errorf("expected config to take precedence over stored state, got %+v", cfg2)
	}
}

func TestManager_SetUsageAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	store := NewFileStore(tempDir)

	m := NewManager("modem_usage_test", Config{SMSLimit: 100, CallMinutesLimit: 60}, nil)
	m.SetStore(store)

	smsMonth := 14
	smsDay := 2
	callsUsed := 3.5
	dataUsed := int64(1048576)

	m.SetUsage(UsageUpdate{
		SMSMonthCount:   &smsMonth,
		SMSDayCount:     &smsDay,
		CallMinutesUsed: &callsUsed,
		DataBytesUsed:   &dataUsed,
	})

	st := m.Status()
	if st.SMSMonthCount != 14 || st.SMSDayCount != 2 || st.CallMinutesUsed != 3.5 || st.DataBytesUsed != 1048576 {
		t.Errorf("unexpected status after SetUsage: %+v", st)
	}
	if st.SMSRemaining != 86 {
		t.Errorf("expected SMSRemaining = 86, got %d", st.SMSRemaining)
	}
	if st.CallMinutesRemaining != 56.5 {
		t.Errorf("expected CallMinutesRemaining = 56.5, got %f", st.CallMinutesRemaining)
	}

	// Verify persistence and restoration without empty reset date wiping counters
	m2 := NewManager("modem_usage_test", Config{SMSLimit: 100, CallMinutesLimit: 60}, nil)
	m2.SetStore(store)
	st2 := m2.Status()
	if st2.SMSMonthCount != 14 || st2.SMSDayCount != 2 || st2.CallMinutesUsed != 3.5 || st2.DataBytesUsed != 1048576 {
		t.Errorf("unexpected restored status: %+v", st2)
	}
}
