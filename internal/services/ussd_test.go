package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/ussd"
)

type mockUSSDSender struct {
	mu      sync.Mutex
	sent    []string
	syncResp string
	err     error
}

func (m *mockUSSDSender) SendUSSD(code string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return "", m.err
	}
	m.sent = append(m.sent, code)
	return m.syncResp, nil
}

func TestUSSDService_Send_InvalidCode(t *testing.T) {
	sender := &mockUSSDSender{}
	svc := NewUSSDService("siemens_tc35", sender, nil)

	_, err := svc.Send(context.Background(), "100")
	if !errors.Is(err, ussd.ErrInvalidUSSDFormat) {
		t.Fatalf("expected ErrInvalidUSSDFormat, got: %v", err)
	}
}

func TestUSSDService_Send_Synchronous(t *testing.T) {
	sender := &mockUSSDSender{
		syncResp: `+CUSD: 0,"Balance is 150 RUB",15`,
	}
	svc := NewUSSDService("siemens_tc35", sender, nil)

	resp, err := svc.Send(context.Background(), "*100#")
	if err != nil {
		t.Fatalf("unexpected Send error: %v", err)
	}
	if resp == nil || resp.Message != "Balance is 150 RUB" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestUSSDService_Send_AsyncURC(t *testing.T) {
	sender := &mockUSSDSender{
		syncResp: "OK",
	}
	var receivedResp *ussd.Response
	svc := NewUSSDService("siemens_tc35", sender, func(r *ussd.Response) {
		receivedResp = r
	})

	// Send in background, then deliver URC
	errCh := make(chan error, 1)
	respCh := make(chan *ussd.Response, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		resp, err := svc.Send(ctx, "*100#")
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	time.Sleep(20 * time.Millisecond)
	svc.HandleURC(`+CUSD: 0,"Balance is 250 RUB",15`)

	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case resp := <-respCh:
		if resp.Message != "Balance is 250 RUB" {
			t.Errorf("expected balance 250 RUB, got: %q", resp.Message)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for async USSD response")
	}

	if receivedResp == nil || receivedResp.Message != "Balance is 250 RUB" {
		t.Errorf("expected callback to receive response, got: %+v", receivedResp)
	}
}
