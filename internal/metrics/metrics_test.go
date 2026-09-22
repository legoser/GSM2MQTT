package metrics

import (
	"strings"
	"sync"
	"testing"
)

func TestMetricsRegistry_Counters(t *testing.T) {
	r := NewRegistry()
	r.IncCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": "sim800", "status": "delivered"})
	r.IncCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": "sim800", "status": "delivered"})
	r.AddCounter("gsm2mqtt_sms_sent_total", map[string]string{"modem": "sim800", "status": "failed"}, 3)

	rendered := r.Render()

	if !strings.Contains(rendered, `gsm2mqtt_sms_sent_total{modem="sim800",status="delivered"} 2`) {
		t.Errorf("expected delivered count 2, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, `gsm2mqtt_sms_sent_total{modem="sim800",status="failed"} 3`) {
		t.Errorf("expected failed count 3, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "# TYPE gsm2mqtt_sms_sent_total counter") {
		t.Errorf("missing TYPE header for counter in:\n%s", rendered)
	}
}

func TestMetricsRegistry_Gauges(t *testing.T) {
	r := NewRegistry()
	r.SetGauge("gsm2mqtt_signal_rssi", map[string]string{"modem": "sim800"}, 18)
	r.SetGauge("gsm2mqtt_balance_rub", map[string]string{"modem": "sim800"}, 125.50)

	rendered := r.Render()

	if !strings.Contains(rendered, `gsm2mqtt_signal_rssi{modem="sim800"} 18`) {
		t.Errorf("expected signal rssi 18, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, `gsm2mqtt_balance_rub{modem="sim800"} 125.5`) {
		t.Errorf("expected balance 125.5, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "# TYPE gsm2mqtt_signal_rssi gauge") {
		t.Errorf("missing TYPE header for gauge in:\n%s", rendered)
	}
}

func TestMetricsRegistry_Concurrency(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				r.IncCounter("concurrency_counter", map[string]string{"worker": "yes"})
				r.SetGauge("concurrency_gauge", map[string]string{"worker": "yes"}, float64(j))
				_ = r.Render()
			}
		}()
	}
	wg.Wait()

	rendered := r.Render()
	expected := workers * iterations
	expectedSub := `concurrency_counter{worker="yes"} 2000`
	if !strings.Contains(rendered, expectedSub) {
		t.Errorf("expected total count %d, rendered output:\n%s", expected, rendered)
	}
}

func TestMetricsRegistry_EdgeCases(t *testing.T) {
	r := NewRegistry()

	t.Run("empty labels", func(t *testing.T) {
		r.IncCounter("uptime_ticks", nil)
		rendered := r.Render()
		if !strings.Contains(rendered, "uptime_ticks 1") {
			t.Errorf("expected metric without labels, got:\n%s", rendered)
		}
	})

	t.Run("label with quotes and newlines sanitized", func(t *testing.T) {
		r.SetGauge("sanitized_metric", map[string]string{"info": "foo\"bar\nbaz\\qux"}, 42)
		rendered := r.Render()
		if strings.Contains(rendered, "foo\"bar") && !strings.Contains(rendered, `foo\"bar`) {
			t.Errorf("quote was not escaped: %s", rendered)
		}
	})
}
