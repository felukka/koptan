package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// receiver records the requests it gets and answers with the given codes,
// one per request (the last one repeats).
type receiver struct {
	mu     sync.Mutex
	codes  []int
	bodies [][]byte
	heads  []http.Header
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.bodies = append(rc.bodies, body)
	rc.heads = append(rc.heads, r.Header.Clone())
	code := http.StatusOK
	if len(rc.codes) > 0 {
		code = rc.codes[0]
		if len(rc.codes) > 1 {
			rc.codes = rc.codes[1:]
		}
	}
	w.WriteHeader(code)
	_, _ = w.Write([]byte("answer"))
}

func event() Event {
	return Event{ID: "ns/api/CISucceeded/x", Kind: "CISucceeded", Service: "api", Namespace: "ns",
		Revision: "0123456789abcdef", Image: "ghcr.io/x/api:0123", Message: "Built it",
		Severity: SeveritySuccess, Time: time.Unix(0, 0)}
}

func send(t *testing.T, typ string, rc *receiver, key string) error {
	t.Helper()
	srv := httptest.NewServer(rc)
	t.Cleanup(srv.Close)
	// httptest serves plain http; chat channels require https, so they are
	// tested through a webhook-typed URL check below and a TLS server here.
	if typ != TypeWebhook {
		srv.Close()
		srv = httptest.NewTLSServer(rc)
		t.Cleanup(srv.Close)
	}
	n, err := New(Config{Type: typ, URL: srv.URL + "/hook", SigningKey: key}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return n.Send(context.Background(), event())
}

func TestSlackPayload(t *testing.T) {
	rc := &receiver{}
	if err := send(t, TypeSlack, rc, ""); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Text   string           `json:"text"`
		Blocks []map[string]any `json:"blocks"`
	}
	if err := json.Unmarshal(rc.bodies[0], &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Text, "CISucceeded ns/api") || len(body.Blocks) != 3 {
		t.Errorf("unexpected Slack payload: %s", rc.bodies[0])
	}
	if !strings.Contains(string(rc.bodies[0]), "0123456789ab") {
		t.Error("the short revision must be shown")
	}
}

func TestTeamsAdaptiveCard(t *testing.T) {
	rc := &receiver{}
	if err := send(t, TypeTeams, rc, ""); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Type        string `json:"type"`
		Attachments []struct {
			ContentType string         `json:"contentType"`
			Content     map[string]any `json:"content"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(rc.bodies[0], &body); err != nil {
		t.Fatal(err)
	}
	if body.Type != "message" || body.Attachments[0].ContentType != "application/vnd.microsoft.card.adaptive" ||
		body.Attachments[0].Content["type"] != "AdaptiveCard" {
		t.Errorf("unexpected Teams payload: %s", rc.bodies[0])
	}
}

func TestWebhookSignsTheBody(t *testing.T) {
	rc := &receiver{}
	if err := send(t, TypeWebhook, rc, "s3cret"); err != nil {
		t.Fatal(err)
	}
	if got, want := rc.heads[0].Get(SignatureHeader), Sign([]byte("s3cret"), rc.bodies[0]); got != want {
		t.Errorf("signature = %q, want %q", got, want)
	}
	if rc.heads[0].Get("X-Koptan-Delivery") != "ns/api/CISucceeded/x" {
		t.Error("the delivery id lets receivers deduplicate")
	}
	var e Event
	if err := json.Unmarshal(rc.bodies[0], &e); err != nil || e.Image != "ghcr.io/x/api:0123" {
		t.Errorf("webhook body must be the event: %v %s", err, rc.bodies[0])
	}
}

func TestRetries(t *testing.T) {
	rc := &receiver{codes: []int{503, 200}}
	if err := send(t, TypeWebhook, rc, ""); err != nil || len(rc.bodies) != 2 {
		t.Errorf("a 503 must be retried: err %v after %d attempts", err, len(rc.bodies))
	}
	rc = &receiver{codes: []int{400}}
	err := send(t, TypeWebhook, rc, "")
	if err == nil || len(rc.bodies) != 1 || !strings.Contains(err.Error(), "400") {
		t.Errorf("a 400 must fail at once: err %v after %d attempts", err, len(rc.bodies))
	}
}

func TestURLsAreCheckedAndNeverLeaked(t *testing.T) {
	if _, err := New(Config{Type: TypeSlack, URL: "http://hooks.slack.com/x"}, nil); err == nil {
		t.Error("chat channels must use https")
	}
	if _, err := New(Config{Type: TypeWebhook, URL: "ftp://x"}, nil); err == nil {
		t.Error("only http(s) URLs are allowed")
	}
	secretURL := "https://hooks.example.invalid/services/T000/B000/XXXXSECRET"
	n, err := New(Config{Type: TypeSlack, URL: secretURL}, &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = n.Send(ctx, event())
	if err == nil || strings.Contains(err.Error(), "XXXXSECRET") {
		t.Errorf("errors must not contain the webhook URL: %v", err)
	}
}

func TestBlockedAddresses(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:80", "[::1]:443", "169.254.169.254:80", "0.0.0.0:80"} {
		if err := refuseBlocked("tcp", addr, nil); !errors.Is(err, errBlockedAddress) {
			t.Errorf("%s: want blocked, got %v", addr, err)
		}
	}
	for _, addr := range []string{"10.0.0.5:80", "192.168.1.2:443", "52.1.2.3:443"} {
		if err := refuseBlocked("tcp", addr, nil); err != nil {
			t.Errorf("%s: want allowed, got %v", addr, err)
		}
	}
	srv := httptest.NewServer(&receiver{})
	defer srv.Close()
	n, _ := New(Config{Type: TypeWebhook, URL: srv.URL}, NewHTTPClient(time.Second))
	if err := n.Send(context.Background(), event()); err == nil {
		t.Error("the operator's client must refuse a loopback receiver")
	}
}
