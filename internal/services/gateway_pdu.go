package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem/at"
)

type atPDUSender struct {
	engine *at.Engine
}

func (s *atPDUSender) SendPDU(ctx context.Context, cmdLength int, pduHex string) (byte, error) {
	resp, err := s.engine.SendPDU(ctx, cmdLength, pduHex, 30*time.Second)
	if err != nil {
		return 0, err
	}
	if resp.Error {
		return 0, fmt.Errorf("PDU send returned error: %v", resp.Lines)
	}
	for _, line := range resp.Lines {
		if strings.HasPrefix(line, "+CMGS:") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				ref, err := strconv.Atoi(parts[1])
				if err == nil {
					return byte(ref), nil
				}
			}
		}
	}
	return 0, nil
}
