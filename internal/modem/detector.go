package modem

import (
	"context"
	"strings"
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
func Detect(ctx context.Context, at ATCommander) (Type, error) {
	var combined strings.Builder

	cmds := []string{"ATI", "AT+CGMI", "AT+CGMM"}
	for _, cmd := range cmds {
		if resp, err := at.SendCommand(ctx, cmd); err == nil {
			combined.WriteString(" ")
			combined.WriteString(resp)
		}
	}

	return classifyModem(combined.String()), nil
}

func classifyModem(text string) Type {
	upper := strings.ToUpper(text)

	siemensKeywords := []string{"SIEMENS", "CINTERION", "TC35", "MC55", "TC65", "MC35"}
	if containsAny(upper, siemensKeywords) {
		return TypeSiemens
	}

	simcomKeywords := []string{"SIMCOM", "SIM800", "SIM900", "SIM7000", "SIM7600", "SIM5320"}
	if containsAny(upper, simcomKeywords) {
		return TypeSIMCom
	}

	huaweiKeywords := []string{"HUAWEI", "E3372", "E3531", "E173", "E1550", "E1750"}
	if containsAny(upper, huaweiKeywords) {
		return TypeHuawei
	}

	quectelKeywords := []string{"QUECTEL", "EC25", "EC21", "M95", "BG96", "MC60"}
	if containsAny(upper, quectelKeywords) {
		return TypeQuectel
	}

	return TypeGeneric
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
