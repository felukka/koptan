import type { DeliveryEntry } from '@internal/plugin-koptan-common';
import { Icon } from '@internal/plugin-koptan-react';

const time = (t: string) => new Date(t).toLocaleString();

/** The latest notifications sent by every Alert, newest first. */
export const DeliveryFeed = ({
  deliveries,
}: {
  deliveries: DeliveryEntry[];
}) => (
  <div style={{ display: 'grid', gap: 10 }}>
    {deliveries.map((d) => (
      <div
        key={`${d.alert}/${d.channel}/${d.time}/${d.event}`}
        style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}
        title={d.error}
      >
        <span
          style={{ color: d.success ? 'var(--mz-success)' : 'var(--mz-error)' }}
        >
          <Icon name={d.success ? 'check_circle' : 'error'} size={18} />
        </span>
        <div>
          <div>
            <strong>{d.event}</strong> · {d.service}
            {d.revision && (
              <span className="mz-code"> {d.revision.slice(0, 12)}</span>
            )}
          </div>
          <div className="mz-label mz-label--dim">
            {d.alert} → {d.channel} · {time(d.time)}
          </div>
          {d.error && <div className="mz-error-text">{d.error}</div>}
        </div>
      </div>
    ))}
  </div>
);
