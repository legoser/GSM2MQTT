package ussd

// Status represents the 3GPP TS 27.007 +CUSD status code.
type Status int

const (
	// StatusCompleted indicates no further user action required (e.g. balance check result).
	StatusCompleted Status = 0

	// StatusActionRequired indicates further user action required (e.g. interactive menu).
	StatusActionRequired Status = 1

	// StatusTerminated indicates session terminated by network.
	StatusTerminated Status = 2

	// StatusOtherClient indicates other local client has responded.
	StatusOtherClient Status = 3

	// StatusNotSupported indicates operation not supported by network.
	StatusNotSupported Status = 4

	// StatusTimeout indicates network timeout.
	StatusTimeout Status = 5
)

// Response represents a parsed USSD reply from the network.
type Response struct {
	// Status is the 3GPP +CUSD status integer (0..5).
	Status Status `json:"status"`

	// Message is the decoded, human-readable text (UTF-8).
	Message string `json:"message"`

	// DCS is the Data Coding Scheme returned by the network (e.g. 15 for GSM-7, 72 for UCS-2).
	DCS int `json:"dcs"`

	// ActionRequired is true if the session is interactive (status 1).
	ActionRequired bool `json:"action_required"`
}
