// Package security provides rate limiting, phone number filtering,
// and AT command sanitization for the GSM gateway.
package security

import "errors"

var (
	// ErrNumberBlocked is returned when a phone number matches the blacklist.
	ErrNumberBlocked = errors.New("phone number is blocked by blacklist")

	// ErrNumberNotWhitelisted is returned when a phone number is not found in the whitelist.
	ErrNumberNotWhitelisted = errors.New("phone number is not in whitelist")

	// ErrInvalidFilterMode is returned when an unsupported filter mode is specified.
	ErrInvalidFilterMode = errors.New("invalid filter mode")

	// ErrEmptyPhoneNumber is returned when a phone number is empty.
	ErrEmptyPhoneNumber = errors.New("phone number cannot be empty")

	// ErrRawATDisabled is returned when raw AT execution is requested but disabled in configuration.
	ErrRawATDisabled = errors.New("raw AT commands are disabled")

	// ErrCommandBlocked is returned when a raw AT command matches the blocked commands list.
	ErrCommandBlocked = errors.New("command is blocked by security policy")

	// ErrEmptyCommand is returned when an AT command is empty.
	ErrEmptyCommand = errors.New("AT command cannot be empty")

	// ErrDangerousChars is returned when external input contains dangerous or control characters.
	ErrDangerousChars = errors.New("input contains dangerous control characters")

	// ErrRateLimitMinuteExceeded is returned when the per-minute SMS rate limit is exceeded.
	ErrRateLimitMinuteExceeded = errors.New("rate limit exceeded: minute limit reached")

	// ErrRateLimitHourExceeded is returned when the per-hour SMS rate limit is exceeded.
	ErrRateLimitHourExceeded = errors.New("rate limit exceeded: hour limit reached")

	// ErrRateLimitDayExceeded is returned when the daily SMS rate limit is exceeded.
	ErrRateLimitDayExceeded = errors.New("rate limit exceeded: day limit reached")

	// ErrRateLimitPerNumberExceeded is returned when the per-recipient SMS limit is exceeded.
	ErrRateLimitPerNumberExceeded = errors.New("rate limit exceeded: limit per recipient reached")

	// ErrRateLimitCooldown is returned when sending is blocked because cooldown period is active.
	ErrRateLimitCooldown = errors.New("rate limit cooldown active")
)
