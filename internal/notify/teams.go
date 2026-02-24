package notify

import (
	"context"
	"encoding/json"
)

// teams posts an Adaptive Card to a Microsoft Teams Workflows webhook.
type teams struct{ poster }

var teamsColor = map[Severity]string{
	SeverityInfo: "Accent", SeveritySuccess: "Good", SeverityError: "Attention",
}

func (t teams) Send(ctx context.Context, e Event) error {
	facts := []map[string]string{}
	for _, f := range e.facts() {
		facts = append(facts, map[string]string{"title": f[0], "value": f[1]})
	}
	card := map[string]any{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body": []map[string]any{
			{"type": "TextBlock", "text": e.Kind, "weight": "Bolder", "size": "Medium",
				"color": teamsColor[e.Severity]},
			{"type": "TextBlock", "text": e.Message, "wrap": true},
			{"type": "FactSet", "facts": facts},
		},
	}
	body, err := json.Marshal(map[string]any{
		"type": "message",
		"attachments": []map[string]any{
			{"contentType": "application/vnd.microsoft.card.adaptive", "content": card},
		},
	})
	if err != nil {
		return err
	}
	return t.post(ctx, body, nil)
}
