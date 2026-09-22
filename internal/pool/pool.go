package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// ModemMember defines the methods required from a modem to be managed in a pool.
type ModemMember interface {
	ID() string
	Status() string
	Signal() int
	Operator() string
	SendSMS(ctx context.Context, to, text string) ([]byte, error)
	SendUSSD(ctx context.Context, code string) (string, error)
	Dial(ctx context.Context, number string) error
	Hangup(ctx context.Context) error
}

// Config specifies pool initialization settings.
type Config struct {
	Strategy     Strategy
	DefaultModem string
}

// ModemInfo provides public status metadata for a single modem in the pool.
type ModemInfo struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Signal   int    `json:"signal"`
	Operator string `json:"operator"`
}

// Status represents the health and distribution state of the modem pool.
type Status struct {
	Strategy    Strategy    `json:"strategy"`
	TotalModems int         `json:"total_modems"`
	ReadyModems int         `json:"ready_modems"`
	Modems      []ModemInfo `json:"modems"`
}

// Pool coordinates a group of modems with automatic load balancing and failover.
type Pool struct {
	mu       sync.RWMutex
	strategy Strategy
	modems   []ModemMember
	rrIndex  uint64
}

// New creates and initializes a new modem Pool.
func New(cfg Config) (*Pool, error) {
	if cfg.Strategy == "" {
		cfg.Strategy = StrategyRoundRobin
	}
	if !cfg.Strategy.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStrategy, cfg.Strategy)
	}

	return &Pool{
		strategy: cfg.Strategy,
		modems:   make([]ModemMember, 0),
	}, nil
}

// Register adds a modem member to the pool.
func (p *Pool) Register(m ModemMember) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, existing := range p.modems {
		if existing.ID() == m.ID() {
			p.modems[i] = m
			return
		}
	}
	p.modems = append(p.modems, m)
}

// Unregister removes a modem member by its ID.
func (p *Pool) Unregister(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, m := range p.modems {
		if m.ID() == id {
			p.modems = append(p.modems[:i], p.modems[i+1:]...)
			return
		}
	}
}

// Select chooses a single candidate modem matching the strategy and criteria.
func (p *Pool) Select(ctx context.Context, criteria SelectionCriteria) (ModemMember, error) {
	candidates, err := p.Candidates(criteria)
	if err != nil {
		return nil, err
	}
	return candidates[0], nil
}

// Candidates returns an ordered list of candidate modems to attempt, for failover.
func (p *Pool) Candidates(criteria SelectionCriteria) ([]ModemMember, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.modems) == 0 {
		return nil, ErrNoReadyModems
	}

	if criteria.RequiredModemID != "" {
		for _, m := range p.modems {
			if m.ID() == criteria.RequiredModemID && m.Status() == "ready" {
				return []ModemMember{m}, nil
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrModemNotFound, criteria.RequiredModemID)
	}

	ready := make([]ModemMember, 0, len(p.modems))
	for _, m := range p.modems {
		if m.Status() == "ready" {
			ready = append(ready, m)
		}
	}
	if len(ready) == 0 {
		return nil, ErrNoReadyModems
	}

	return p.orderCandidates(ready, criteria), nil
}

func (p *Pool) orderCandidates(ready []ModemMember, criteria SelectionCriteria) []ModemMember {
	switch p.strategy {
	case StrategyFailover:
		return ready
	case StrategyBestSignal:
		best := pickBestSignal(ready)
		return prependCandidate(ready, best)
	case StrategyOperatorMatch:
		matched := pickOperatorMatch(ready, criteria.RecipientNumber)
		return prependCandidate(ready, matched)
	case StrategyRoundRobin:
		fallthrough
	default:
		idx := atomic.AddUint64(&p.rrIndex, 1) - 1
		start := int(idx % uint64(len(ready)))
		res := make([]ModemMember, len(ready))
		for i := 0; i < len(ready); i++ {
			res[i] = ready[(start+i)%len(ready)]
		}
		return res
	}
}

func prependCandidate(list []ModemMember, first ModemMember) []ModemMember {
	res := make([]ModemMember, 0, len(list))
	res = append(res, first)
	for _, m := range list {
		if m.ID() != first.ID() {
			res = append(res, m)
		}
	}
	return res
}

// SendSMS attempts to send SMS, performing automatic failover across candidates if needed.
func (p *Pool) SendSMS(ctx context.Context, to, text string) ([]byte, error) {
	candidates, err := p.Candidates(SelectionCriteria{RecipientNumber: to})
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, m := range candidates {
		ref, err := m.SendSMS(ctx, to, text)
		if err == nil {
			return ref, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("%w: %v", ErrAllModemsFailed, lastErr)
}

// SendUSSD dispatches a USSD command to a suitable modem.
func (p *Pool) SendUSSD(ctx context.Context, code string) (string, error) {
	m, err := p.Select(ctx, SelectionCriteria{})
	if err != nil {
		return "", err
	}
	return m.SendUSSD(ctx, code)
}

// Dial initiates a voice call on a ready modem.
func (p *Pool) Dial(ctx context.Context, number string) error {
	candidates, err := p.Candidates(SelectionCriteria{RecipientNumber: number})
	if err != nil {
		return err
	}
	var lastErr error
	for _, m := range candidates {
		if err := m.Dial(ctx, number); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("%w: %v", ErrAllModemsFailed, lastErr)
}

// Hangup terminates voice calls across all modems in the pool.
func (p *Pool) Hangup(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var firstErr error
	for _, m := range p.modems {
		if err := m.Hangup(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Status returns summary snapshot of the pool and its members.
func (p *Pool) Status() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()

	readyCount := 0
	infos := make([]ModemInfo, len(p.modems))
	for i, m := range p.modems {
		st := m.Status()
		if st == "ready" {
			readyCount++
		}
		infos[i] = ModemInfo{
			ID:       m.ID(),
			Status:   st,
			Signal:   m.Signal(),
			Operator: m.Operator(),
		}
	}

	return Status{
		Strategy:    p.strategy,
		TotalModems: len(p.modems),
		ReadyModems: readyCount,
		Modems:      infos,
	}
}
