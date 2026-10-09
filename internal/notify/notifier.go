package notify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Notifier sends events to one destination.
type Notifier interface {
	Send(ctx context.Context, e Event) error
}

// Channel types, matching the Alert API.
const (
	TypeSlack   = "slack"
	TypeTeams   = "teams"
	TypeWebhook = "webhook"
)

// Config describes one destination.
type Config struct {
	Type string
	URL  string
	// SigningKey signs webhook bodies; ignored by chat channels.
	SigningKey string
}

// New returns the Notifier for cfg, posting with client.
func New(cfg Config, client *http.Client) (Notifier, error) {
	if err := validateURL(cfg); err != nil {
		return nil, err
	}
	p := poster{url: cfg.URL, client: client}
	switch cfg.Type {
	case TypeSlack:
		return slack{p}, nil
	case TypeTeams:
		return teams{p}, nil
	case TypeWebhook:
		return webhook{poster: p, key: []byte(cfg.SigningKey)}, nil
	}
	return nil, fmt.Errorf("unknown channel type %q", cfg.Type)
}

// validateURL accepts https, and plain http for generic webhooks only
// (typically a receiver inside the cluster).
func validateURL(cfg Config) error {
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("the %s URL is not a valid URL", cfg.Type)
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && cfg.Type == TypeWebhook:
		return nil
	}
	return fmt.Errorf("the %s URL must use https", cfg.Type)
}

// poster sends JSON bodies with a few retries on transient failures.
type poster struct {
	url    string
	client *http.Client
}

const attempts = 3

func (p poster) post(ctx context.Context, body []byte, headers map[string]string) error {
	var last error
	for i := range attempts {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i) * 500 * time.Millisecond):
			}
		}
		retry, err := p.once(ctx, body, headers)
		if err == nil {
			return nil
		}
		last = err
		if !retry {
			break
		}
	}
	return last
}

// once posts the body; retry reports whether a failure may be transient.
func (p poster) once(ctx context.Context, body []byte, headers map[string]string) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "koptan-operator")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		// Never echo the URL: chat webhook URLs are credentials.
		return true, fmt.Errorf("post failed: %v", redact(err, p.url))
	}
	defer func() { _ = resp.Body.Close() }()
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return false, nil
	}
	err = fmt.Errorf("receiver answered %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500, err
}

// redact removes the URL from an error message.
func redact(err error, u string) string {
	return string(bytes.ReplaceAll([]byte(err.Error()), []byte(u), []byte("<url>")))
}
