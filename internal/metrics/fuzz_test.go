package metrics

import (
	"testing"
)

func FuzzRegistry_FormatLabels(f *testing.F) {
	f.Add("key1", "simple_val", "key2", "val with \"quotes\" and \n newline")
	f.Add("modem", "sim800", "status", "delivered")
	f.Add("empty", "", "special", `\backslash\`)

	f.Fuzz(func(t *testing.T, k1, v1, k2, v2 string) {
		if k1 == "" || k2 == "" {
			return
		}
		labels := map[string]string{k1: v1, k2: v2}
		formatted := formatLabels(labels)
		if len(formatted) == 0 {
			t.Errorf("expected non-empty formatted labels")
		}

		r := NewRegistry()
		r.SetGauge("fuzz_metric", labels, 1.23)
		rendered := r.Render()
		if len(rendered) == 0 {
			t.Errorf("rendered string should not be empty")
		}
	})
}
