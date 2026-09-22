package modem

import (
	"context"
)

// Type represents a recognized GSM modem family.
type Type string

const (
	TypeGeneric Type = "generic"
	TypeSiemens Type = "siemens"
	TypeSIMCom  Type = "simcom"
	TypeHuawei  Type = "huawei"
	TypeQuectel Type = "quectel"
)

// ATCommander sends an AT command and returns the response string.
type ATCommander interface {
	SendCommand(ctx context.Context, cmd string) (string, error)
}

// Detect queries the modem using identification AT commands (ATI, AT+CGMI, AT+CGMM)
// and returns the identified modem Type.
// Unimplemented stub for TDD.
func Detect(ctx context.Context, at ATCommander) (Type, error) {
	// STUB for TDD: will fail tests
	return TypeGeneric, nil
}
