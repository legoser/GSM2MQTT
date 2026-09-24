// Package operator provides mobile network operator presets, USSD balance query
// configurations, and balance response parsing.
package operator

import "errors"

var (
	// ErrEmptyResponse is returned when the balance response string is empty.
	ErrEmptyResponse = errors.New("balance response is empty")

	// ErrBalanceNotFound is returned when no monetary balance could be parsed from the response.
	ErrBalanceNotFound = errors.New("balance pattern not found in response")

	// ErrInvalidBalanceFormat is returned when the matched balance substring cannot be converted to a valid number.
	ErrInvalidBalanceFormat = errors.New("invalid balance number format")

	// ErrInvalidBalanceRegex is returned when a custom balance regex is rejected (too long).
	ErrInvalidBalanceRegex = errors.New("invalid custom balance regex")

	// ErrUnknownPreset is returned when an unknown operator preset name is requested.
	ErrUnknownPreset = errors.New("unknown operator preset")
)
