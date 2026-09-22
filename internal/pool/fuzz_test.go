package pool

import (
	"context"
	"fmt"
	"testing"
)

func FuzzPool_Selection(f *testing.F) {
	f.Add("+79101234567", 0, 3)
	f.Add("+79269876543", 1, 2)
	f.Add("+79031112233", 2, 4)
	f.Add("unknown", 3, 1)

	strategies := []Strategy{StrategyRoundRobin, StrategyFailover, StrategyBestSignal, StrategyOperatorMatch}

	f.Fuzz(func(t *testing.T, recipient string, stratIdx int, modemCount int) {
		if modemCount <= 0 || modemCount > 20 {
			return
		}
		strat := strategies[abs(stratIdx)%len(strategies)]

		p, err := New(Config{Strategy: strat})
		if err != nil {
			t.Fatalf("unexpected pool error: %v", err)
		}

		for i := 0; i < modemCount; i++ {
			st := "ready"
			if i%3 == 0 && i > 0 {
				st = "error"
			}
			p.Register(&mockMember{
				id:       fmt.Sprintf("modem_%d", i),
				status:   st,
				signal:   (i * 5) % 31,
				operator: "MTS",
			})
		}

		sel, err := p.Select(context.Background(), SelectionCriteria{RecipientNumber: recipient})
		if err != nil && err != ErrNoReadyModems {
			t.Fatalf("unexpected select error: %v", err)
		}
		if sel != nil && sel.Status() != "ready" {
			t.Errorf("selected non-ready modem: %v", sel)
		}
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
