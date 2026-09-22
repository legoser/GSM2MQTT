package ussd

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/legoser/gsm2mqtt/internal/sms/pdu"
)

// ValidateCode checks that the USSD code is not empty and conforms to the standard USSD format.
func ValidateCode(code string) error {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ErrEmptyUSSDCode
	}

	if len(trimmed) < 3 {
		return ErrInvalidUSSDFormat
	}

	// Must start with * or # and end with #
	if (!strings.HasPrefix(trimmed, "*") && !strings.HasPrefix(trimmed, "#")) || !strings.HasSuffix(trimmed, "#") {
		return ErrInvalidUSSDFormat
	}

	for _, r := range trimmed {
		if (r < '0' || r > '9') && r != '*' && r != '#' {
			return ErrInvalidUSSDFormat
		}
	}

	return nil
}

// ParseResponse parses and decodes a +CUSD unsolicited result code or response string.
func ParseResponse(urc string) (*Response, error) {
	trimmed := strings.TrimSpace(urc)
	if !strings.HasPrefix(trimmed, "+CUSD:") {
		return nil, fmt.Errorf("%w: missing +CUSD prefix", ErrMalformedUSSDResponse)
	}

	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "+CUSD:"))
	if body == "" {
		return nil, fmt.Errorf("%w: empty body", ErrMalformedUSSDResponse)
	}

	// Parse status code: first token before comma or end of string
	var statusRaw string
	var rest string
	if idx := strings.IndexByte(body, ','); idx >= 0 {
		statusRaw = strings.TrimSpace(body[:idx])
		rest = strings.TrimSpace(body[idx+1:])
	} else {
		statusRaw = body
	}

	statusCode, err := strconv.Atoi(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid status integer %q: %v", ErrMalformedUSSDResponse, statusRaw, err)
	}

	switch Status(statusCode) {
	case StatusTerminated:
		return nil, ErrUSSDTerminated
	case StatusNotSupported:
		return nil, ErrUSSDNotSupported
	case StatusTimeout:
		return nil, ErrUSSDTimeout
	case StatusCompleted, StatusActionRequired, StatusOtherClient:
		// Normal response flow
	default:
		return nil, fmt.Errorf("%w: unknown status code %d", ErrMalformedUSSDResponse, statusCode)
	}

	var message string
	dcs := 15 // Default GSM 7-bit

	if rest != "" {
		// Extract quoted string if present
		startQuote := strings.IndexByte(rest, '"')
		if startQuote >= 0 {
			endQuote := strings.LastIndexByte(rest, '"')
			if endQuote > startQuote {
				message = rest[startQuote+1 : endQuote]
				afterQuote := strings.TrimSpace(rest[endQuote+1:])
				if strings.HasPrefix(afterQuote, ",") {
					dcsStr := strings.TrimSpace(strings.TrimPrefix(afterQuote, ","))
					if parsedDCS, err := strconv.Atoi(dcsStr); err == nil {
						dcs = parsedDCS
					}
				}
			}
		}
	}

	// Decode UCS-2 hex if DCS == 72 or if message is valid hex with length % 4 == 0
	if (dcs == 72 || len(message)%4 == 0) && len(message) >= 4 && isHex(message) {
		if rawBytes, err := hex.DecodeString(message); err == nil {
			decoded := pdu.DecodeUCS2(rawBytes)
			if decoded != "" && isPrintable(decoded) {
				message = decoded
			}
		}
	} else {
		// Unescape any escaped newline sequences common in USSD menus
		message = strings.ReplaceAll(message, "\\n", "\n")
		message = strings.ReplaceAll(message, "\\r", "\r")
	}

	return &Response{
		Status:         Status(statusCode),
		Message:        message,
		DCS:            dcs,
		ActionRequired: Status(statusCode) == StatusActionRequired,
	}, nil
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func isHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
