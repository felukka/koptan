import type { Alert, ChannelType } from '@internal/plugin-koptan-common';
import { Badge, Icon } from '@internal/plugin-koptan-react';

export const CHANNEL_ICON: Record<ChannelType, string> = {
  slack: 'tag',
  teams: 'groups',
  webhook: 'webhook',
};

/** What the Alert watches: one Service or a label selector. */
const target = (a: Alert) =>
  a.spec.serviceRef?.name ??
  Object.entries(a.spec.selector?.matchLabels ?? {})
    .map(([k, v]) => `${k}=${v}`)
    .join(', ');

const ready = (a: Alert) =>
  a.status?.conditions?.find((c) => c.type === 'Ready');

export const List = ({ alerts }: { alerts: Alert[] }) => (
  <>
    {alerts.map((a) => {
      const cond = ready(a);
      const ok = cond?.status === 'True';
      return (
        <div
          className="mz-item"
          key={`${a.metadata.namespace}/${a.metadata.name}`}
        >
          <div style={{ display: 'grid', gap: 6 }}>
            <div className="mz-item-title">
              {a.metadata.name}{' '}
              <span className="mz-muted">→ {target(a) || 'nothing'}</span>
            </div>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {(a.spec.events?.length ? a.spec.events : ['All events']).map(
                (e) => (
                  <Badge key={e}>{e}</Badge>
                ),
              )}
            </div>
            <div className="mz-label mz-label--dim">
              {a.spec.channels.map((ch) => (
                <span key={ch.name} style={{ marginRight: 12 }}>
                  <Icon name={CHANNEL_ICON[ch.type]} size={14} /> {ch.name}
                </span>
              ))}
            </div>
          </div>
          <span title={cond?.message}>
            <Badge
              variant={a.spec.suspend ? 'warning' : ok ? 'success' : 'error'}
            >
              {a.spec.suspend ? 'Suspended' : (cond?.reason ?? 'Pending')}
            </Badge>
          </span>
        </div>
      );
    })}
  </>
);
