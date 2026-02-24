import type {
  ChannelType,
  CreateAlertRequest,
} from '@internal/plugin-koptan-common';
import { Icon } from '@internal/plugin-koptan-react';

export type ChannelDraft = CreateAlertRequest['channels'][number];

const PLACEHOLDER: Record<ChannelType, string> = {
  slack: 'https://hooks.slack.com/services/…',
  teams: 'https://…logic.azure.com/workflows/…',
  webhook: 'https://receiver.example.com/koptan',
};

/** Rows of channel name, type, URL and (webhooks) signing key. */
export const ChannelEditor = ({
  rows,
  onChange,
}: {
  rows: ChannelDraft[];
  onChange: (rows: ChannelDraft[]) => void;
}) => {
  const set = (i: number, patch: Partial<ChannelDraft>) =>
    onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  return (
    <div className="mz-list-edit">
      {rows.map((ch, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: rows are positional
        <div key={i} style={{ display: 'grid', gap: 8 }}>
          <div className="mz-list-row">
            <input
              className="mz-input"
              aria-label="Channel name"
              value={ch.name}
              placeholder="name, e.g. ops"
              onChange={(e) => set(i, { name: e.target.value })}
            />
            <select
              className="mz-input"
              aria-label="Channel type"
              value={ch.type}
              onChange={(e) =>
                set(i, {
                  type: e.target.value as ChannelType,
                  signingKey: undefined,
                })
              }
            >
              <option value="slack">Slack</option>
              <option value="teams">Teams</option>
              <option value="webhook">Webhook</option>
            </select>
            <button
              type="button"
              className="mz-btn"
              aria-label="Remove channel"
              onClick={() => onChange(rows.filter((_, j) => j !== i))}
            >
              <Icon name="close" size={16} />
            </button>
          </div>
          <input
            className="mz-input"
            aria-label="Webhook URL"
            type="password"
            autoComplete="off"
            value={ch.url}
            placeholder={PLACEHOLDER[ch.type]}
            onChange={(e) => set(i, { url: e.target.value })}
          />
          {ch.type === 'webhook' && (
            <input
              className="mz-input"
              aria-label="Signing key"
              type="password"
              autoComplete="off"
              value={ch.signingKey ?? ''}
              placeholder="signing key (optional, HMAC-SHA256)"
              onChange={(e) =>
                set(i, { signingKey: e.target.value || undefined })
              }
            />
          )}
        </div>
      ))}
      <button
        type="button"
        className="mz-btn"
        onClick={() =>
          onChange([...rows, { name: '', type: 'slack', url: '' }])
        }
      >
        <Icon name="add" size={16} /> Add channel
      </button>
    </div>
  );
};
