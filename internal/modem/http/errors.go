// Package http provides direct HTTP/HTTPS client capabilities executed
// entirely through the modem's embedded AT command IP stack (e.g. SIMCom/Quectel HTTP stack)
// without requiring OS-level PPP or network daemon configuration.
package http

import "errors"

var (
	// ErrEmptyURL is returned when an HTTP request specifies an empty URL.
	ErrEmptyURL = errors.New("request URL cannot be empty")

	// ErrUnsupportedMethod is returned when a method other than GET or POST is requested.
	ErrUnsupportedMethod = errors.New("unsupported HTTP method (only GET and POST supported via AT)")

	// ErrBearerFailed is returned when activating the GPRS data bearer fails.
	ErrBearerFailed = errors.New("failed to activate GPRS bearer")

	// ErrHTTPRequestFailed is returned when the modem reports a failure during HTTP action.
	ErrHTTPRequestFailed = errors.New("modem HTTP action failed")
)
