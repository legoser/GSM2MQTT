package sms

import (
	"fmt"
	"strconv"
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

// AssembledSMS represents a complete or flushed multipart received SMS message.
type AssembledSMS struct {
	From       string    `json:"from"`
	Text       string    `json:"text"`
	Timestamp  time.Time `json:"timestamp"`
	Segments   int       `json:"segments"`
	Encoding   string    `json:"encoding"`
	IsComplete bool      `json:"is_complete"`
	IsUpdate   bool      `json:"is_update,omitempty"`
}

type partialMessage struct {
	from       string
	parts      map[int]string
	totalParts int
	timestamp  time.Time
	encoding   string
	createdAt  time.Time
	timer      *time.Timer
}

type flushedMessage struct {
	from       string
	parts      map[int]string
	totalParts int
	timestamp  time.Time
	encoding   string
	flushedAt  time.Time
}

// Assembler handles reassembly of multipart SMS messages with timeout and late-part fallback.
type Assembler struct {
	mu      sync.Mutex
	ttl     time.Duration
	timeout time.Duration
	pending map[string]*partialMessage
	flushed map[string]*flushedMessage
	onFlush func(msg *AssembledSMS)
}

// NewAssembler creates a new SMS Assembler with the given part expiration TTL and optional assembly timeout.
func NewAssembler(ttl time.Duration, timeouts ...time.Duration) *Assembler {
	var timeout time.Duration
	if len(timeouts) > 0 {
		timeout = timeouts[0]
	}
	return &Assembler{
		ttl:     ttl,
		timeout: timeout,
		pending: make(map[string]*partialMessage),
		flushed: make(map[string]*flushedMessage),
	}
}

// SetFlushHandler registers a callback invoked when an incomplete multipart message times out and is flushed.
func (a *Assembler) SetFlushHandler(handler func(msg *AssembledSMS)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onFlush = handler
}

// SetAssemblyTimeout configures the timeout after which an incomplete multipart message is flushed.
func (a *Assembler) SetAssemblyTimeout(timeout time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.timeout = timeout
}

// Close cancels all pending assembly timers and clears state.
func (a *Assembler) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, pm := range a.pending {
		if pm.timer != nil {
			pm.timer.Stop()
			pm.timer = nil
		}
	}
	a.pending = make(map[string]*partialMessage)
	a.flushed = make(map[string]*flushedMessage)
}

// AddPart processes an incoming SMS part.
// If the message is complete (or not multipart, or completes an earlier flushed message),
// it returns the assembled message and true.
// If more parts are expected, it starts/resets the assembly timer and returns nil, false.
func (a *Assembler) AddPart(part IncomingPart) (*AssembledSMS, bool) {
	if !part.IsMultipart || part.TotalParts <= 1 {
		return &AssembledSMS{
			From:       part.From,
			Text:       part.Text,
			Timestamp:  part.Timestamp,
			Segments:   1,
			Encoding:   part.Encoding,
			IsComplete: true,
		}, true
	}

	// Boundary check: invalid part indices or excessively large total parts
	if part.PartNumber <= 0 || part.PartNumber > part.TotalParts || part.TotalParts > 20 {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cleanupExpired()

	key := fmt.Sprintf("%s:%d", part.From, part.Reference)

	// Late-arrival fallback: Check if this part belongs to a message that already timed out and was flushed
	if fm, ok := a.flushed[key]; ok {
		fm.parts[part.PartNumber] = part.Text
		if len(fm.parts) == fm.totalParts {
			var b strings.Builder
			for i := 1; i <= fm.totalParts; i++ {
				b.WriteString(fm.parts[i])
			}
			delete(a.flushed, key)
			return &AssembledSMS{
				From:       fm.from,
				Text:       "[Updated] " + b.String(),
				Timestamp:  fm.timestamp,
				Segments:   fm.totalParts,
				Encoding:   fm.encoding,
				IsComplete: true,
				IsUpdate:   true,
			}, true
		}

		// Still incomplete, but return updated progress
		rangeStr := formatPartsSummary(fm.parts, fm.totalParts)
		text := fmt.Sprintf("[Updated %s]: %s", rangeStr, buildAvailableText(fm.parts, fm.totalParts))
		return &AssembledSMS{
			From:       fm.from,
			Text:       text,
			Timestamp:  fm.timestamp,
			Segments:   len(fm.parts),
			Encoding:   fm.encoding,
			IsComplete: false,
			IsUpdate:   true,
		}, true
	}

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
		if pm.timer != nil {
			pm.timer.Stop()
			pm.timer = nil
		}
		var b strings.Builder
		for i := 1; i <= pm.totalParts; i++ {
			b.WriteString(pm.parts[i])
		}

		delete(a.pending, key)

		return &AssembledSMS{
			From:       pm.from,
			Text:       b.String(),
			Timestamp:  pm.timestamp,
			Segments:   pm.totalParts,
			Encoding:   pm.encoding,
			IsComplete: true,
		}, true
	}

	// Arm or extend the assembly timeout timer
	if a.timeout > 0 {
		if pm.timer != nil {
			pm.timer.Stop()
		}
		pm.timer = time.AfterFunc(a.timeout, func() {
			a.flushPending(key)
		})
	}

	return nil, false
}

func (a *Assembler) flushPending(key string) {
	a.mu.Lock()
	pm, ok := a.pending[key]
	if !ok {
		a.mu.Unlock()
		return
	}
	delete(a.pending, key)

	fm := &flushedMessage{
		from:       pm.from,
		parts:      pm.parts,
		totalParts: pm.totalParts,
		timestamp:  pm.timestamp,
		encoding:   pm.encoding,
		flushedAt:  time.Now(),
	}
	a.flushed[key] = fm
	onFlush := a.onFlush
	a.mu.Unlock()

	rangeStr := formatPartsSummary(fm.parts, fm.totalParts)
	text := fmt.Sprintf("[%s]: %s", rangeStr, buildAvailableText(fm.parts, fm.totalParts))

	assembled := &AssembledSMS{
		From:       fm.from,
		Text:       text,
		Timestamp:  fm.timestamp,
		Segments:   len(fm.parts),
		Encoding:   fm.encoding,
		IsComplete: false,
	}

	if onFlush != nil {
		onFlush(assembled)
	}
}

func formatPartsSummary(parts map[int]string, totalParts int) string {
	var nums []int
	for i := 1; i <= totalParts; i++ {
		if _, ok := parts[i]; ok {
			nums = append(nums, i)
		}
	}
	if len(nums) == 0 {
		return fmt.Sprintf("Part ?/%d", totalParts)
	}
	if isContiguous(nums) {
		if len(nums) == 1 {
			return fmt.Sprintf("Part %d/%d", nums[0], totalParts)
		}
		return fmt.Sprintf("Part %d-%d/%d", nums[0], nums[len(nums)-1], totalParts)
	}
	strs := make([]string, len(nums))
	for i, n := range nums {
		strs[i] = strconv.Itoa(n)
	}
	return fmt.Sprintf("Part %s/%d", strings.Join(strs, ","), totalParts)
}

func isContiguous(nums []int) bool {
	for i := 1; i < len(nums); i++ {
		if nums[i] != nums[i-1]+1 {
			return false
		}
	}
	return true
}

func buildAvailableText(parts map[int]string, totalParts int) string {
	var b strings.Builder
	for i := 1; i <= totalParts; i++ {
		if text, ok := parts[i]; ok {
			b.WriteString(text)
		} else {
			// Only insert gap indicator if there are received parts both before and after this missing slot
			hasEarlier := false
			for j := 1; j < i; j++ {
				if _, ok := parts[j]; ok {
					hasEarlier = true
					break
				}
			}
			hasLater := false
			for j := i + 1; j <= totalParts; j++ {
				if _, ok := parts[j]; ok {
					hasLater = true
					break
				}
			}
			if hasEarlier && hasLater {
				b.WriteString(" [...] ")
			}
		}
	}
	return b.String()
}

func (a *Assembler) cleanupExpired() {
	now := time.Now()
	if a.ttl > 0 {
		for key, pm := range a.pending {
			if now.Sub(pm.createdAt) > a.ttl {
				if pm.timer != nil {
					pm.timer.Stop()
				}
				delete(a.pending, key)
			}
		}
	}

	flushedTTL := 15 * time.Minute
	if a.ttl > 0 && a.ttl < flushedTTL {
		flushedTTL = a.ttl
	}
	for key, fm := range a.flushed {
		if now.Sub(fm.flushedAt) > flushedTTL {
			delete(a.flushed, key)
		}
	}
}
