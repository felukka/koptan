package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// SignatureHeader carries sha256=<hex HMAC of the body> when a signing key
// is configured; receivers recompute it to check the sender.
const SignatureHeader = "X-Koptan-Signature"

// webhook posts the event itself as JSON.
type webhook struct {
	poster
	key []byte
}

func (w webhook) Send(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	headers := map[string]string{"X-Koptan-Event": e.Kind, "X-Koptan-Delivery": e.ID}
	if len(w.key) > 0 {
		headers[SignatureHeader] = Sign(w.key, body)
	}
	return w.post(ctx, body, headers)
}

// Sign returns the X-Koptan-Signature value for body.
func Sign(key, body []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
