// Package notify delivers pipeline events to Slack, Microsoft Teams and
// generic webhooks. Each channel type lives in its own file and only
// formats the payload; posting, retries and network safety are shared.
package notify

import (
	"fmt"
	"time"
)

// Severity colours a notification.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
	SeverityError   Severity = "error"
)

// Event is one thing that happened to a Service's pipeline.
type Event struct {
	// ID is stable for one occurrence, so receivers can deduplicate.
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Service   string    `json:"service"`
	Namespace string    `json:"namespace"`
	Repo      string    `json:"repo,omitempty"`
	Revision  string    `json:"revision,omitempty"`
	Image     string    `json:"image,omitempty"`
	Phase     string    `json:"phase,omitempty"`
	Message   string    `json:"message"`
	Severity  Severity  `json:"severity"`
	Time      time.Time `json:"time"`
}

// Title is a one-line summary for chat channels.
func (e Event) Title() string {
	return fmt.Sprintf("%s %s: %s", e.Kind, e.Namespace+"/"+e.Service, e.Message)
}

// ShortRevision is the first 12 characters of the commit SHA.
func (e Event) ShortRevision() string {
	if len(e.Revision) > 12 {
		return e.Revision[:12]
	}
	return e.Revision
}

// facts are the key/value lines chat cards show under the title.
func (e Event) facts() [][2]string {
	out := [][2]string{{"Service", e.Namespace + "/" + e.Service}}
	for _, f := range [][2]string{
		{"Revision", e.ShortRevision()}, {"Image", e.Image}, {"Phase", e.Phase}, {"Repository", e.Repo},
	} {
		if f[1] != "" {
			out = append(out, f)
		}
	}
	return out
}
