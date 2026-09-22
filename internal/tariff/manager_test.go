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
	mu.Lock()
	if len(alerts) != 1 {
		t.Errorf("expected duplicate alert to be suppressed, got %d alerts", len(alerts))
	}
	mu.Unlock()

	// Recover above threshold
	m.UpdateBalance(100.0, "RUB")

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
