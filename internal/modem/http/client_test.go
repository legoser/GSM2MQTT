package http

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem/at"
)

type mockATRunner struct {
	mu        sync.Mutex
	commands  []string
	responses map[string]*at.Response
}

func newMockATRunner() *mockATRunner {
	return &mockATRunner{
		responses: make(map[string]*at.Response),
	}
}

func (m *mockATRunner) Send(cmd string, timeout time.Duration) (*at.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clean := strings.TrimSpace(cmd)
	m.commands = append(m.commands, clean)

	if resp, ok := m.responses[clean]; ok {
		return resp, nil
	}
	return &at.Response{OK: true}, nil
}

func (m *mockATRunner) SendCommand(ctx context.Context, cmd string) (string, error) {
	resp, err := m.Send(cmd, time.Second)
	if err != nil {
		return "", err
	}
	return strings.Join(resp.Lines, "\n"), nil
}

func TestClient_Get_Success(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+HTTPACTION=0"] = &at.Response{
		OK:    true,
		Lines: []string{"+HTTPACTION: 0,200,12"},
	}
	runner.responses["AT+HTTPREAD"] = &at.Response{
		OK:    true,
		Lines: []string{"+HTTPREAD: 12", "Hello World!"},
	}

	client := NewClient(runner, "internet")
	resp, err := client.Get(context.Background(), "http://api.example.com/status")
	if err != nil {
		t.Fatalf("unexpected Get error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "Hello World!" {
		t.Errorf("expected body 'Hello World!', got %q", string(resp.Body))
	}
}

func TestClient_Post_Success(t *testing.T) {
	runner := newMockATRunner()
	runner.responses["AT+HTTPACTION=1"] = &at.Response{
		OK:    true,
		Lines: []string{"+HTTPACTION: 1,200,15"},
	}
	runner.responses["AT+HTTPREAD"] = &at.Response{
		OK:    true,
		Lines: []string{"+HTTPREAD: 15", `{"ok":true}`},
	}

	client := NewClient(runner, "internet")
	payload := []byte(`{"text":"alert"}`)
	resp, err := client.Post(context.Background(), "https://api.telegram.org/bot/send", "application/json", payload)
	if err != nil {
		t.Fatalf("unexpected Post error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("expected body '{\"ok\":true}', got %q", string(resp.Body))
	}
}

func TestClient_InvalidInputs(t *testing.T) {
	runner := newMockATRunner()
	client := NewClient(runner, "internet")

	// Empty URL
	_, err := client.Get(context.Background(), "")
	if !errors.Is(err, ErrEmptyURL) {
		t.Errorf("expected ErrEmptyURL, got: %v", err)
	}

	// Unsupported Method
	_, err = client.Do(context.Background(), Request{
		Method: "DELETE",
		URL:    "http://api.example.com",
	})
	if !errors.Is(err, ErrUnsupportedMethod) {
		t.Errorf("expected ErrUnsupportedMethod, got: %v", err)
	}
}
