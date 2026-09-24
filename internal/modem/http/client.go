package http

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/legoser/gsm2mqtt/internal/modem/at"
)

// ATRunner executes AT commands.
type ATRunner interface {
	Send(cmd string, timeout time.Duration) (*at.Response, error)
	SendCommand(ctx context.Context, cmd string) (string, error)
}

// Request represents an outgoing HTTP request executed via modem AT stack.
type Request struct {
	Method      string
	URL         string
	ContentType string
	Body        []byte
	Timeout     time.Duration
}

// Response represents the HTTP response received via modem.
type Response struct {
	StatusCode int
	Body       []byte
}

// Client executes HTTP requests directly using modem embedded AT command IP stack.
type Client struct {
	runner ATRunner
	apn    string
	mu     sync.Mutex
}

// NewClient creates a new modem HTTP Client.
func NewClient(runner ATRunner, apn string) *Client {
	if apn == "" {
		apn = "internet"
	}
	return &Client{
		runner: runner,
		apn:    apn,
	}
}

// Get executes an HTTP GET request.
func (c *Client) Get(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, Request{
		Method: "GET",
		URL:    url,
	})
}

// Post executes an HTTP POST request.
func (c *Client) Post(ctx context.Context, url string, contentType string, body []byte) (*Response, error) {
	return c.Do(ctx, Request{
		Method:      "POST",
		URL:         url,
		ContentType: contentType,
		Body:        body,
	})
}

// Do executes the given HTTP request via AT commands.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	if strings.TrimSpace(req.URL) == "" {
		return nil, ErrEmptyURL
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method != "GET" && method != "POST" {
		return nil, ErrUnsupportedMethod
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	defer func() {
		_, _ = c.runner.Send("AT+HTTPTERM", 3*time.Second)
	}()

	if err := c.initSession(req, method); err != nil {
		return nil, err
	}

	statusCode, err := c.executeAction(method)
	if err != nil {
		return nil, err
	}

	body := c.readResponseBody()
	return &Response{
		StatusCode: statusCode,
		Body:       body,
	}, nil
}

func (c *Client) initSession(req Request, method string) error {
	_, _ = c.runner.Send("AT+HTTPINIT", 3*time.Second)
	_, _ = c.runner.Send("AT+HTTPPARA=\"CID\",1", 3*time.Second)

	urlCmd := fmt.Sprintf("AT+HTTPPARA=\"URL\",%q", req.URL)
	if resp, err := c.runner.Send(urlCmd, 3*time.Second); err != nil {
		return fmt.Errorf("failed to set URL: %w", err)
	} else if resp.Error {
		return fmt.Errorf("failed to set URL: modem returned ERROR (%v)", resp.Lines)
	}

	if method == "POST" {
		ct := req.ContentType
		if ct == "" {
			ct = "application/json"
		}
		ctCmd := fmt.Sprintf("AT+HTTPPARA=\"CONTENT\",%q", ct)
		_, _ = c.runner.Send(ctCmd, 3*time.Second)

		dataCmd := fmt.Sprintf("AT+HTTPDATA=%d,10000\r%s", len(req.Body), string(req.Body))
		_, _ = c.runner.Send(dataCmd, 10*time.Second)
	}

	return nil
}

func (c *Client) executeAction(method string) (int, error) {
	actionNum := 0
	if method == "POST" {
		actionNum = 1
	}

	actionCmd := fmt.Sprintf("AT+HTTPACTION=%d", actionNum)
	resp, err := c.runner.Send(actionCmd, 30*time.Second)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
	}

	for _, line := range resp.Lines {
		if strings.HasPrefix(line, "+HTTPACTION:") {
			parts := strings.Split(strings.TrimPrefix(line, "+HTTPACTION:"), ",")
			if len(parts) >= 2 {
				code, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil {
					return code, nil
				}
			}
		}
	}
	return 200, nil
}

func (c *Client) readResponseBody() []byte {
	readResp, err := c.runner.Send("AT+HTTPREAD", 10*time.Second)
	if err != nil || readResp.Error || len(readResp.Lines) == 0 {
		return nil
	}

	var bodyLines []string
	for _, l := range readResp.Lines {
		if !strings.HasPrefix(l, "+HTTPREAD:") {
			bodyLines = append(bodyLines, l)
		}
	}
	return []byte(strings.Join(bodyLines, "\n"))
}
