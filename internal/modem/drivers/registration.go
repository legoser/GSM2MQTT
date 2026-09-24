package drivers

import (
	"strconv"
	"strings"

	"github.com/legoser/gsm2mqtt/internal/modem"
)

// parseRegLine parses +CREG or +CGREG response lines to determine registration status.
func parseRegLine(lines []string, prefix string, defaultTech string) *modem.NetworkStatus {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			body := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			parts := strings.Split(body, ",")
			statIdx := 1
			if len(parts) == 1 {
				statIdx = 0
			}
			if len(parts) > statIdx {
				stat, err := strconv.Atoi(strings.TrimSpace(parts[statIdx]))
				if err == nil {
					return &modem.NetworkStatus{
						Registered: stat == 1 || stat == 5,
						Roaming:    stat == 5,
						Technology: defaultTech,
					}
				}
			}
		}
	}
	return nil
}
