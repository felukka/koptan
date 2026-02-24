package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// slack posts to a Slack incoming webhook with Block Kit.
type slack struct{ poster }

var slackEmoji = map[Severity]string{
	SeverityInfo: ":information_source:", SeveritySuccess: ":white_check_mark:", SeverityError: ":x:",
}

func (s slack) Send(ctx context.Context, e Event) error {
	var fields strings.Builder
	for _, f := range e.facts() {
		fmt.Fprintf(&fields, "*%s:* %s\n", f[0], f[1])
	}
	body, err := json.Marshal(map[string]any{
		"text": e.Title(),
		"blocks": []map[string]any{
			{"type": "section", "text": map[string]string{
				"type": "mrkdwn", "text": fmt.Sprintf("%s *%s* — %s", slackEmoji[e.Severity], e.Kind, e.Message),
			}},
			{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": fields.String()}},
			{"type": "context", "elements": []map[string]string{
				{"type": "mrkdwn", "text": "Koptan · " + e.Time.UTC().Format("2006-01-02 15:04:05 MST")},
			}},
		},
	})
	if err != nil {
		return err
	}
	return s.post(ctx, body, nil)
}
