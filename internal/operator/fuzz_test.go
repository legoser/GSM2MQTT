package operator

import (
	"testing"
)

func FuzzParseBalance(f *testing.F) {
	seeds := []string{
		"Баланс: 152.40 руб.",
		"Баланс: 152,40р.",
		"Vash balans: -15.50 r.",
		"Balance: 50.00 RUB",
		"Остаток: 250.75 ₽",
		"",
		"   ",
		"0.00",
		"-100",
		"123456789.99",
		"Text without numbers",
		"Balans: 10,20,30",
		"Balans: NaN руб",
	}

	for _, s := range seeds {
		f.Add(s, "")
	}

	f.Fuzz(func(t *testing.T, text string, pattern string) {
		// ParseBalance should never panic under any arbitrary input
		_, _ = ParseBalance(text, pattern)
	})
}
