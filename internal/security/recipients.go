package security

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var digitsOnlyRegex = regexp.MustCompile(`^[0-9]+$`)

// NormalizeNumber formats an input phone number into canonical E.164 international format.
func NormalizeNumber(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("phone number cannot be empty")
	}

	// Remove common punctuation: spaces, dashes, parentheses
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '(', ')', '.', '\t', '\r', '\n':
			return -1
		default:
			return r
		}
	}, trimmed)

	hasPlus := strings.HasPrefix(cleaned, "+")
	digits := strings.TrimPrefix(cleaned, "+")

	if !digitsOnlyRegex.MatchString(digits) {
		return "", fmt.Errorf("invalid phone number format: %q", raw)
	}

	if len(digits) < 4 || len(digits) > 16 {
		return "", fmt.Errorf("invalid phone number length: %q", raw)
	}

	// Russian national format: 8900... -> +7900...
	if !hasPlus && len(digits) == 11 && strings.HasPrefix(digits, "8") {
		return "+7" + digits[1:], nil
	}

	return "+" + digits, nil
}

// RecipientsManager maintains a persistent, thread-safe collection of authorized alert recipients.
type RecipientsManager struct {
	filePath string
	mu       sync.RWMutex
	numbers  []string
	onChange func([]string)
}

// NewRecipientsManager creates a RecipientsManager and loads existing numbers from disk if available.
func NewRecipientsManager(filePath string, defaultNumbers []string) *RecipientsManager {
	m := &RecipientsManager{
		filePath: filePath,
		numbers:  make([]string, 0),
	}

	if err := m.load(); err != nil || len(m.numbers) == 0 {
		for _, raw := range defaultNumbers {
			if norm, err := NormalizeNumber(raw); err == nil {
				m.addNumberInMemory(norm)
			}
		}
		if len(m.numbers) > 0 && filePath != "" {
			_ = m.save()
		}
	}

	return m
}

// SetOnChange registers a callback triggered whenever the list of recipients changes.
func (m *RecipientsManager) SetOnChange(fn func([]string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// Get returns a thread-safe copy of all registered recipient phone numbers.
func (m *RecipientsManager) Get() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, len(m.numbers))
	copy(res, m.numbers)
	return res
}

// Contains checks if the given phone number is registered.
func (m *RecipientsManager) Contains(raw string) bool {
	norm, err := NormalizeNumber(raw)
	if err != nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, n := range m.numbers {
		if n == norm {
			return true
		}
	}
	return false
}

// Add appends a phone number, normalizes it, persists to disk, and triggers onChange.
func (m *RecipientsManager) Add(raw string) error {
	norm, err := NormalizeNumber(raw)
	if err != nil {
		slog.Warn("failed to add alert recipient: invalid phone format",
			slog.String("input", raw),
			slog.Any("error", err),
		)
		return err
	}

	m.mu.Lock()
	for _, n := range m.numbers {
		if n == norm {
			m.mu.Unlock()
			slog.Debug("alert recipient already registered", slog.String("number", norm))
			return nil
		}
	}

	m.numbers = append(m.numbers, norm)
	_ = m.saveLocked()
	fn := m.onChange
	cpy := m.copyLocked()
	count := len(m.numbers)
	m.mu.Unlock()

	slog.Info("alert recipient added successfully",
		slog.String("number", norm),
		slog.Int("total_recipients", count),
	)

	if fn != nil {
		fn(cpy)
	}
	return nil
}

// Remove deletes a phone number from the registry, persists to disk, and triggers onChange.
func (m *RecipientsManager) Remove(raw string) bool {
	norm, err := NormalizeNumber(raw)
	if err != nil {
		slog.Warn("failed to remove alert recipient: invalid phone format",
			slog.String("input", raw),
			slog.Any("error", err),
		)
		return false
	}

	m.mu.Lock()
	idx := -1
	for i, n := range m.numbers {
		if n == norm {
			idx = i
			break
		}
	}

	if idx == -1 {
		m.mu.Unlock()
		slog.Warn("alert recipient not found for removal", slog.String("number", norm))
		return false
	}

	m.numbers = append(m.numbers[:idx], m.numbers[idx+1:]...)
	_ = m.saveLocked()
	fn := m.onChange
	cpy := m.copyLocked()
	count := len(m.numbers)
	m.mu.Unlock()

	slog.Info("alert recipient removed",
		slog.String("number", norm),
		slog.Int("total_recipients", count),
	)

	if fn != nil {
		fn(cpy)
	}
	return true
}

// Set replaces the complete list of recipients with new numbers.
func (m *RecipientsManager) Set(rawNumbers []string) error {
	var normalized []string
	seen := make(map[string]bool)

	for _, raw := range rawNumbers {
		norm, err := NormalizeNumber(raw)
		if err != nil {
			slog.Warn("invalid number in alert recipients list", slog.String("input", raw), slog.Any("error", err))
			return err
		}
		if !seen[norm] {
			seen[norm] = true
			normalized = append(normalized, norm)
		}
	}

	m.mu.Lock()
	m.numbers = normalized
	_ = m.saveLocked()
	fn := m.onChange
	cpy := m.copyLocked()
	count := len(m.numbers)
	m.mu.Unlock()

	masked := make([]string, len(normalized))
	for i, n := range normalized {
		masked[i] = MaskPhone(n)
	}

	slog.Info("alert recipients list updated",
		slog.Int("total_recipients", count),
		slog.Any("numbers", masked),
	)

	if fn != nil {
		fn(cpy)
	}
	return nil
}

func (m *RecipientsManager) copyLocked() []string {
	cpy := make([]string, len(m.numbers))
	copy(cpy, m.numbers)
	return cpy
}

func (m *RecipientsManager) addNumberInMemory(norm string) {
	for _, n := range m.numbers {
		if n == norm {
			return
		}
	}
	m.numbers = append(m.numbers, norm)
}

func (m *RecipientsManager) load() error {
	if m.filePath == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var nums []string
	if err := json.Unmarshal(data, &nums); err != nil {
		return err
	}

	for _, raw := range nums {
		if norm, err := NormalizeNumber(raw); err == nil {
			m.addNumberInMemory(norm)
		}
	}
	if len(m.numbers) > 0 {
		slog.Info("loaded alert recipients from disk",
			slog.String("path", m.filePath),
			slog.Int("count", len(m.numbers)),
		)
	}
	return nil
}

func (m *RecipientsManager) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

func (m *RecipientsManager) saveLocked() error {
	if m.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.numbers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0600)
}
