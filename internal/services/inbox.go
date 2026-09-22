package services

import (
	"fmt"
	"time"

	"github.com/legoser/gsm2mqtt/internal/sms"
)

func (r *ModemRunner) recordIncomingSMS(msg *sms.AssembledSMS) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := ReceivedSMS{
		ID:        fmt.Sprintf("%s-%d", r.mCfg.ID, time.Now().UnixNano()),
		ModemID:   r.mCfg.ID,
		Sender:    msg.From,
		Timestamp: msg.Timestamp.Format("2006-01-02 15:04:05"),
		Text:      msg.Text,
	}
	r.receivedSMS = append(r.receivedSMS, entry)
	if len(r.receivedSMS) > 50 {
		r.receivedSMS = r.receivedSMS[len(r.receivedSMS)-50:]
	}
}

// GetReceivedSMS returns the list of recently received SMS messages for this modem.
func (r *ModemRunner) GetReceivedSMS() []ReceivedSMS {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]ReceivedSMS, len(r.receivedSMS))
	copy(res, r.receivedSMS)
	return res
}
