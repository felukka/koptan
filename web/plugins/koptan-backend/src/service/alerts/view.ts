import type {
  Alert,
  DeliveryEntry,
  SignalMast,
} from '@internal/plugin-koptan-common';
import type { KoptanClient, RawResource } from '../k8s';
import { cleanMetadata } from '../pipelines';

/** Drops the dedupe state, which is bookkeeping, not something to show. */
export function redactAlert(raw: RawResource): Alert {
  const { lastNotified: _ln, ...status } = raw.status ?? {};
  return { ...raw, metadata: cleanMetadata(raw), status } as Alert;
}

/** Every Alert's deliveries in one feed, newest first. */
export function buildDeliveries(alerts: Alert[], limit = 50): DeliveryEntry[] {
  return alerts
    .flatMap((a) =>
      (a.status?.deliveries ?? []).map((d) => ({
        ...d,
        alert: a.metadata.name,
        namespace: a.metadata.namespace,
      })),
    )
    .sort((a, b) => b.time.localeCompare(a.time))
    .slice(0, limit);
}

export async function loadSignalMast(
  client: KoptanClient,
  namespace?: string,
): Promise<SignalMast> {
  const alerts = (await client.list('Alert', namespace)).map(redactAlert);
  alerts.sort((a, b) => a.metadata.name.localeCompare(b.metadata.name));
  return { alerts, deliveries: buildDeliveries(alerts) };
}
