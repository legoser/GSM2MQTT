package sms

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for phone number and message validation.
var (
	ErrEmptyPhoneNumber       = errors.New("phone number is empty")
	ErrPhoneNumberTooShort    = errors.New("phone number is too short")
	ErrPhoneNumberTooLong     = errors.New("phone number exceeds E.164 maximum 15 digits")
	ErrInvalidPhoneCharacters = errors.New("phone number contains invalid non-digit characters")
	ErrEmptyText              = errors.New("message text cannot be empty")
	ErrNullByteInText         = errors.New("message text contains null byte \\x00")
	ErrModemEscapeChar        = errors.New("message text contains dangerous modem escape character \\x1A (Ctrl+Z)")
)

// NormalizePhoneNumber validates and normalizes a phone number to E.164 or short code format.
// Strips formatting characters (spaces, dashes, parentheses, dots).
// Replaces national prefix 8 with +7 for 11-digit Russian numbers.
func NormalizePhoneNumber(raw string, defaultCountryPrefix string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrEmptyPhoneNumber
	}

	hasPlus := false
	var digits []rune

	for i, r := range trimmed {
		if r == '+' {
			if i == 0 {
				hasPlus = true
			} else {
				return "", fmt.Errorf("%w: '+' only allowed at start", ErrInvalidPhoneCharacters)
			}
		} else if r >= '0' && r <= '9' {
			digits = append(digits, r)
		} else if r == ' ' || r == '-' || r == '(' || r == ')' || r == '.' {
			continue
		} else {
			return "", fmt.Errorf("%w: unexpected character %q", ErrInvalidPhoneCharacters, r)
		}
	}

	if len(digits) == 0 {
		return "", ErrEmptyPhoneNumber
	}
	if len(digits) < 2 {
		return "", ErrPhoneNumberTooShort
	}

	digitStr := string(digits)

	if hasPlus {
		if len(digits) < 3 {
			return "", ErrPhoneNumberTooShort
		}
		if len(digits) > 15 {
			return "", ErrPhoneNumberTooLong
		}
		return "+" + digitStr, nil
	}

	// Leading 8 national format (11 digits) -> convert to +7
	if len(digits) == 11 && digits[0] == '8' {
		return "+7" + digitStr[1:], nil
	}

	// Short codes (2..6 digits)
	if len(digits) <= 6 {
		return digitStr, nil
	}

	// 10 digits without prefix -> prepend default country prefix
	if len(digits) == 10 && defaultCountryPrefix != "" {
		cleanPrefix := strings.TrimSpace(defaultCountryPrefix)
		if !strings.HasPrefix(cleanPrefix, "+") {
			cleanPrefix = "+" + cleanPrefix
		}
		return cleanPrefix + digitStr, nil
	}

	if len(digits) > 15 {
		return "", ErrPhoneNumberTooLong
	}

	return "+" + digitStr, nil
}

// ValidateMessageText checks message text for invalid or modem-breaking characters.
func ValidateMessageText(text string) error {
	if text == "" {
		return ErrEmptyText
	}

	for _, r := range text {
		if r == 0x00 {
			return ErrNullByteInText
		}
		if r == 0x1A {
			return ErrModemEscapeChar
		}
	}

	return nil
}
