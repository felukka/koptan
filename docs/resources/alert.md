# Alert

An **Alert** sends notifications about a Service's pushes, builds and deployments to Slack, Microsoft Teams or any HTTP endpoint.

- **API:** `koptan.felukka.org/v1`, kind `Alert`
- **Plural:** `alerts` (`kubectl get alerts`)
- **Channels:** `slack`, `teams`, `webhook`

## Example

Webhook URLs are credentials, so they are kept in a Secret, never in the Alert itself:

```bash
kubectl create secret generic example-alert-channels \
  --from-literal=slack-url=https://hooks.slack.com/services/... \
  --from-literal=teams-url=https://prod-00.westeurope.logic.azure.com/... \
  --from-literal=hook-url=http://receiver.tools.svc/koptan \
  --from-literal=hook-signing=change-me
```

```yaml
apiVersion: koptan.felukka.org/v1
kind: Alert
metadata:
  name: example-alert
  namespace: default
spec:
  serviceRef:
    name: example-service
  # Or every Service with a label:
  # selector:
  #   matchLabels:
  #     team: payments
  # Empty means all of: Push, CIStarted, CISucceeded, CIFailed,
  # CDDeploying, CDSucceeded, CDFailed.
  events: [Push, CISucceeded, CIFailed, CDSucceeded, CDFailed]
  channels:
    - name: slack
      type: slack
      urlSecretRef: {name: example-alert-channels, key: slack-url}
    - name: teams
      type: teams
      urlSecretRef: {name: example-alert-channels, key: teams-url}
    - name: hook
      type: webhook
      urlSecretRef: {name: example-alert-channels, key: hook-url}
      # Adds X-Koptan-Signature: sha256=<HMAC of the body>.
      signingSecretRef: {name: example-alert-channels, key: hook-signing}
```

## Events

| Event | Sent when | Message | Severity |
|---|---|---|---|
| `Push` | Koptan sees a new commit on the Service's branch | `New revision <sha> of <repo>` | info |
| `CIStarted` | A build starts | `Building <sha>` | info |
| `CISucceeded` | A build pushes its image | `Built <image>` | success |
| `CIFailed` | A build fails | `Build failed: <reason>` | error |
| `CDDeploying` | A rollout starts | `Rolling out <image>` | info |
| `CDSucceeded` | A rollout completes | `<image> is running (<n> replicas available)` | success |
| `CDFailed` | A rollout stops progressing | `Deployment failed: <reason>` | error |

Leave `events` empty to get all of them. A common setup is failures only:

```yaml
events: [CIFailed, CDFailed]
```

### Delivery rules

- **Each event is sent once per channel.** Koptan records what it sent in `status.lastNotified`, so restarts of the operator do not send duplicates.
- **No history on creation.** When you create an Alert, Koptan records the current state without sending it. You get notified about what happens next.
- **Failed deliveries are retried** every 30 seconds until they succeed.
- **`suspend: true`** stops sending. Events that happen while suspended are skipped, not queued.

## Channels

### Slack

Create an [incoming webhook](https://api.slack.com/messaging/webhooks) in Slack and store its URL. Messages use Slack Block Kit, coloured by severity.

### Microsoft Teams

Create an incoming webhook (a Teams Workflow "Post to a channel when a webhook request is received", or a legacy connector) and store its URL. Messages are Adaptive Cards.

### Webhook

Koptan sends an HTTP `POST` with a JSON body:

```json
{
  "id": "default/example-service/CISucceeded/3f9c2a…@2/example-service-ci-build-x7k2p",
  "kind": "CISucceeded",
  "service": "example-service",
  "namespace": "default",
  "repo": "https://github.com/example/repo.git",
  "revision": "3f9c2a1b7d4e8a0c9b6e5f4d3c2b1a0987654321",
  "image": "ghcr.io/example/repo:3f9c2a1b7d4e",
  "phase": "Succeeded",
  "message": "Built ghcr.io/example/repo:3f9c2a1b7d4e",
  "severity": "success",
  "time": "2026-10-10T12:00:00Z"
}
```

`severity` is `info`, `success` or `error`. Headers:

| Header | Value |
|---|---|
| `Content-Type` | `application/json` |
| `X-Koptan-Event` | The event kind, such as `CIFailed`. |
| `X-Koptan-Delivery` | A unique ID for the event; the same ID on a retry. Use it to drop duplicates. |
| `X-Koptan-Signature` | `sha256=<hex HMAC-SHA256 of the raw body>`, when `signingSecretRef` is set. |

**Verifying the signature** (Python):

```python
import hmac, hashlib

def verify(body: bytes, header: str, key: bytes) -> bool:
    expected = "sha256=" + hmac.new(key, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, header)
```

Node.js:

```js
import { createHmac, timingSafeEqual } from 'node:crypto';

function verify(body, header, key) {
  const expected = 'sha256=' + createHmac('sha256', key).update(body).digest('hex');
  return header.length === expected.length && timingSafeEqual(Buffer.from(header), Buffer.from(expected));
}
```

### URL rules

- Slack and Teams URLs must use `https`. Webhooks may use `http`, for receivers inside the cluster.
- Koptan refuses loopback, link-local (such as cloud metadata at `169.254.169.254`) and unspecified addresses, even after DNS resolution and redirects. In-cluster and private addresses are allowed.

## Spec reference

| Field | Type | Required | Description |
|---|---|---|---|
| `serviceRef.name` | string | One of `serviceRef` / `selector` | One Service in the Alert's namespace. |
| `selector` | LabelSelector | One of `serviceRef` / `selector` | All Services in the namespace with matching labels. |
| `events` | list of event | No | Events to send. Empty means all. |
| `channels` | list of [channel](#channel) | **Yes** (1–10) | Destinations. |
| `suspend` | bool | No | Stop sending. |

### Channel

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string (DNS label) | **Yes** | Identifies the channel in the status. Unique in the Alert. |
| `type` | `slack` / `teams` / `webhook` | **Yes** | Kind of destination. |
| `urlSecretRef` | `{name, key}` | **Yes** | Secret key with the webhook URL. |
| `signingSecretRef` | `{name, key}` | No | `webhook` only: key used to sign each body. |

## Status

```bash
kubectl get alerts
kubectl get alert example-alert -o jsonpath='{range .status.deliveries[*]}{.time} {.event} {.channel} {.success} {.error}{"\n"}{end}'
```

| Field | Description |
|---|---|
| `deliveries` | The 20 most recent attempts, newest first: `time`, `service`, `event`, `channel`, `revision`, `success`, `error`. |
| `lastNotified` | What was last sent per `<service>/<event>/<channel>`. |
| `initialized` | `true` once the state at creation is recorded. |
| `conditions[Ready]` | Whether the Alert is working. Shows errors such as a missing Secret. |

## Creating an Alert from the UI

On the **Alerting** page, choose **New Alert**. You pick a Service (or a label selector), the events and the channels, and paste the webhook URLs. The UI stores the URLs and signing keys in a Secret named `<alert>-channels`, owned by the Alert. See [Koptan UI](../ui/overview.md#alerting).
