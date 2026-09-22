package modem

import (
	"context"
	"strings"
	"testing"
)

type mockCommander struct {
	responses map[string]string
}

func (m *mockCommander) SendCommand(ctx context.Context, cmd string) (string, error) {
	cleanCmd := strings.TrimSpace(cmd)
	if resp, ok := m.responses[cleanCmd]; ok {
		return resp, nil
	}
	return "OK", nil
}

func TestDetect_Siemens(t *testing.T) {
	cmd := &mockCommander{
		responses: map[string]string{
			"ATI":     "SIEMENS\r\nTC35\r\nREVISION 03.01",
			"AT+CGMI": "SIEMENS",
			"AT+CGMM": "TC35",
		},
	}

	modemType, err := Detect(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modemType != TypeSiemens {
		t.Errorf("expected TypeSiemens, got %v", modemType)
	}
}

func TestDetect_SIMCom(t *testing.T) {
	cmd := &mockCommander{
		responses: map[string]string{
			"ATI":     "SIM800 R14.18",
			"AT+CGMI": "SIMCOM_Ltd",
			"AT+CGMM": "SIMCOM_SIM800L",
		},
	}

	modemType, err := Detect(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modemType != TypeSIMCom {
		t.Errorf("expected TypeSIMCom, got %v", modemType)
	}
}

func TestDetect_Huawei(t *testing.T) {
	cmd := &mockCommander{
		responses: map[string]string{
			"ATI":     "Manufacturer: Huawei Technologies Co., Ltd.\r\nModel: E3372",
			"AT+CGMI": "Huawei Technologies Co., Ltd.",
			"AT+CGMM": "E3372",
		},
	}

	modemType, err := Detect(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modemType != TypeHuawei {
		t.Errorf("expected TypeHuawei, got %v", modemType)
	}
}

func TestDetect_GenericFallback(t *testing.T) {
	cmd := &mockCommander{
		responses: map[string]string{
			"ATI":     "UNKNOWN MODEM",
			"AT+CGMI": "Generic Manufacturer",
			"AT+CGMM": "Generic Model",
		},
	}

	modemType, err := Detect(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modemType != TypeGeneric {
		t.Errorf("expected TypeGeneric for unknown modem, got %v", modemType)
	}
}
