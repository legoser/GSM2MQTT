package sms

import "testing"

func TestTransliterate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple cyrillic",
			input:    "Привет мир",
			expected: "Privet mir",
		},
		{
			name:     "mixed case and tricky letters",
			input:    "Щётка и Юла",
			expected: "Shchyotka i Yula",
		},
		{
			name:     "pure latin remains unchanged",
			input:    "Hello World 123!",
			expected: "Hello World 123!",
		},
		{
			name:     "alarm notification message",
			input:    "Сработала сигнализация в гараже",
			expected: "Srabotala signalizatsiya v garazhe",
		},
		{
			name:     "soft and hard signs",
			input:    "Подъём и ночь",
			expected: "Pod'yom i noch'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Transliterate(tt.input)
			if actual != tt.expected {
				t.Errorf("Transliterate(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}
