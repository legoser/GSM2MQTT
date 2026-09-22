// Package tariff provides SMS quota counting, traffic accounting,
// and low balance alert monitoring for GSM modems.
package tariff

import "errors"

var (
	// ErrQuotaExceeded is returned when attempting to send an SMS after monthly limit is reached.
	ErrQuotaExceeded = errors.New("monthly SMS quota exceeded")
)
