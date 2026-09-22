package operator

import (
	"strings"
)

// Preset contains balance query configuration for a specific mobile network operator.
type Preset struct {
	Name         string
	Title        string
	USSDCode     string
	BalanceRegex string
	Currency     string
}

var standardPresets = map[string]*Preset{
	"mts": {
		Name:         "mts",
		Title:        "MTS",
		USSDCode:     "*100#",
		BalanceRegex: `(?i)(?:баланс|balance|balans|остаток)[:\s]*([+-]?\d+[.,]?\d*)`,
		Currency:     "RUB",
	},
	"megafon": {
		Name:         "megafon",
		Title:        "MegaFon",
		USSDCode:     "*100#",
		BalanceRegex: `(?i)(?:баланс|balance|balans|остаток)[:\s]*([+-]?\d+[.,]?\d*)`,
		Currency:     "RUB",
	},
	"beeline": {
		Name:         "beeline",
		Title:        "Beeline",
		USSDCode:     "*102#",
		BalanceRegex: `(?i)(?:баланс|balance|balans|остаток)[:\s]*([+-]?\d+[.,]?\d*)`,
		Currency:     "RUB",
	},
	"tele2": {
		Name:         "tele2",
		Title:        "Tele2",
		USSDCode:     "*105#",
		BalanceRegex: `(?i)(?:баланс|balance|balans|остаток)[:\s]*([+-]?\d+[.,]?\d*)`,
		Currency:     "RUB",
	},
	"generic": {
		Name:         "generic",
		Title:        "Generic Operator",
		USSDCode:     "*100#",
		BalanceRegex: `(?i)(?:баланс|balance|balans|остаток|schete)?[:\s]*([+-]?\d+[.,]?\d*)`,
		Currency:     "RUB",
	},
}

// GetPreset returns the Preset configuration by operator name key.
func GetPreset(name string) (*Preset, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if p, ok := standardPresets[key]; ok {
		return p, nil
	}
	return nil, ErrUnknownPreset
}

// DetectPreset returns the matching preset based on operator name returned by network registration.
func DetectPreset(operatorName string) *Preset {
	upper := strings.ToUpper(strings.TrimSpace(operatorName))

	switch {
	case strings.Contains(upper, "MTS") || strings.Contains(upper, "МТС"):
		return standardPresets["mts"]
	case strings.Contains(upper, "MEGAFON") || strings.Contains(upper, "МЕГАФОН"):
		return standardPresets["megafon"]
	case strings.Contains(upper, "BEELINE") || strings.Contains(upper, "БИЛАЙН"):
		return standardPresets["beeline"]
	case strings.Contains(upper, "TELE2") || strings.Contains(upper, "ТЕЛЕ2") || strings.Contains(upper, "Т2"):
		return standardPresets["tele2"]
	default:
		return standardPresets["generic"]
	}
}
