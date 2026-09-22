package sms

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// IncomingPart represents a single received SMS part that may belong to a multipart message.
type IncomingPart struct {
	From        string
	Text        string
	Timestamp   time.Time
	IsMultipart bool
	Reference   byte
	PartNumber  int
	TotalParts  int
	Encoding    string
}

// AssembledSMS represents a complete received SMS message.
type AssembledSMS struct {
	From      string
	Text      string
	Timestamp time.Time
	Segments  int
	Encoding  string
}

type partialMessage struct {
	from       string
	parts      map[int]string
	totalParts int
	timestamp  time.Time
	encoding   string
	createdAt  time.Time
}

// Assembler handles reassembly of multipart SMS messages.
type Assembler struct {
	mu      sync.Mutex
	ttl     time.Duration
	pending map[string]*partialMessage
}

// NewAssembler creates a new SMS Assembler with the given part expiration TTL.
func NewAssembler(ttl time.Duration) *Assembler {
	return &Assembler{
		ttl:     ttl,
		pending: make(map[string]*partialMessage),
	}
}

// AddPart processes an incoming SMS part.
// If the message is complete (or not multipart), it returns the assembled message and true.
// If more parts are expected, it returns nil and false.
func (a *Assembler) AddPart(part IncomingPart) (*AssembledSMS, bool) {
	if !part.IsMultipart || part.TotalParts <= 1 {
		return &AssembledSMS{
			From:      part.From,
			Text:      part.Text,
			Timestamp: part.Timestamp,
			Segments:  1,
			Encoding:  part.Encoding,
		}, true
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cleanupExpired()

	key := fmt.Sprintf("%s:%d", part.From, part.Reference)
	pm, ok := a.pending[key]
	if !ok {
		pm = &partialMessage{
			from:       part.From,
			parts:      make(map[int]string, part.TotalParts),
			totalParts: part.TotalParts,
			timestamp:  part.Timestamp,
			encoding:   part.Encoding,
			createdAt:  time.Now(),
		}
		a.pending[key] = pm
	}

	pm.parts[part.PartNumber] = part.Text

	if len(pm.parts) == pm.totalParts {
		var b strings.Builder
		for i := 1; i <= pm.totalParts; i++ {
			b.WriteString(pm.parts[i])
		}

		delete(a.pending, key)

		return &AssembledSMS{
			From:      pm.from,
			Text:      b.String(),
			Timestamp: pm.timestamp,
			Segments:  pm.totalParts,
			Encoding:  pm.encoding,
		}, true
	}

	return nil, false
}

func (a *Assembler) cleanupExpired() {
	if a.ttl <= 0 {
		return
	}
	now := time.Now()
	for key, pm := range a.pending {
		if now.Sub(pm.createdAt) > a.ttl {
			delete(a.pending, key)
		}
	}
}
