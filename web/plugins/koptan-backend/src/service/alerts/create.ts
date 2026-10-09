import {
  ALERT_EVENTS,
  type Alert,
  type CreateAlertRequest,
} from '@internal/plugin-koptan-common';
import { createWithSecrets, type KoptanClient } from '../k8s';
import { DNS_LABEL, validateName } from '../pipelines';
import { redactAlert } from './view';

const LABEL_KEY =
  /^([a-z0-9.-]{1,253}\/)?[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$/;
const LABEL_VALUE = /^([A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?)?$/;

/** Mirrors the operator: https for chat, http(s) for webhooks. */
function validChannelUrl(type: string, url: string): boolean {
  try {
    const u = new URL(url);
    return (
      !!u.hostname &&
      (u.protocol === 'https:' ||
        (u.protocol === 'http:' && type === 'webhook'))
    );
  } catch {
    return false;
  }
}

export function validateCreateAlert(
  req: CreateAlertRequest,
): string | undefined {
  const named = validateName(req);
  if (named) return named;
  const labels = Object.entries(req.selector ?? {});
  if (!req.service && !labels.length)
    return 'choose a service or a label selector';
  if (req.service && !DNS_LABEL.test(req.service))
    return 'service must be a Service name';
  for (const [k, v] of labels) {
    if (!LABEL_KEY.test(k) || !LABEL_VALUE.test(v))
      return `selector ${k}=${v} is not a valid label`;
  }
  for (const e of req.events ?? []) {
    if (!ALERT_EVENTS.includes(e)) return `unknown event "${e}"`;
  }
  if (!req.channels?.length || req.channels.length > 10)
    return 'add one to ten channels';
  const names = new Set<string>();
  for (const ch of req.channels) {
    if (!DNS_LABEL.test(ch.name ?? ''))
      return 'channel names must be DNS labels';
    if (names.has(ch.name)) return `channel "${ch.name}" is listed twice`;
    names.add(ch.name);
    if (!['slack', 'teams', 'webhook'].includes(ch.type))
      return `channel "${ch.name}" has an unknown type`;
    if (!validChannelUrl(ch.type, ch.url ?? ''))
      return ch.type === 'webhook'
        ? `channel "${ch.name}" needs an http(s) URL`
        : `channel "${ch.name}" needs an https URL`;
    if (ch.signingKey && ch.type !== 'webhook')
      return 'only webhook channels can sign';
  }
  return undefined;
}

/**
 * Creates the Alert. Channel URLs (and webhook signing keys) are stored in
 * Secret `<name>-channels`, owned by the Alert, and only referenced.
 */
export async function createAlert(
  client: KoptanClient,
  req: CreateAlertRequest,
  createdBy: string,
): Promise<Alert> {
  const namespace = req.namespace ?? 'default';
  const secret = `${req.name}-channels`;
  const data: Record<string, string> = {};
  const channels = req.channels.map((ch) => {
    data[`${ch.name}-url`] = ch.url;
    if (ch.signingKey) data[`${ch.name}-signing`] = ch.signingKey;
    return {
      name: ch.name,
      type: ch.type,
      urlSecretRef: { name: secret, key: `${ch.name}-url` },
      ...(ch.signingKey
        ? { signingSecretRef: { name: secret, key: `${ch.name}-signing` } }
        : {}),
    };
  });
  const selector = req.selector && Object.keys(req.selector).length;
  const alert = await createWithSecrets(
    client,
    'Alert',
    namespace,
    [{ name: secret, data }],
    {
      metadata: {
        name: req.name,
        namespace,
        annotations: { 'koptan.felukka.org/created-by': createdBy },
      },
      spec: {
        ...(req.service ? { serviceRef: { name: req.service } } : {}),
        ...(selector ? { selector: { matchLabels: req.selector } } : {}),
        ...(req.events?.length ? { events: req.events } : {}),
        channels,
      },
    },
  );
  return redactAlert(alert);
}
