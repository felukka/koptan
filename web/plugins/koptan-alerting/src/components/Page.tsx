import { Progress } from '@backstage/core-components';
import {
  Icon,
  KoptanPage,
  Section,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { useState } from 'react';
import useAsyncRetry from 'react-use/esm/useAsyncRetry';
import mast from '../signal-mast.png';
import { List } from './List';
import { Feed } from './Feed';
import { New } from './New';

const Row = ({ label, value }: { label: string; value: string | number }) => (
  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
    <span className="mz-muted">{label}</span>
    <span style={{ fontWeight: 700 }}>{value}</span>
  </div>
);

export const Page = () => {
  const { getSignalMast } = useKoptanApi();
  const { value, loading, error, retry } = useAsyncRetry(getSignalMast, [
    getSignalMast,
  ]);
  const [creating, setCreating] = useState(false);
  const delivered = value?.deliveries.filter((d) => d.success).length ?? 0;
  const failed = (value?.deliveries.length ?? 0) - delivered;

  return (
    <KoptanPage
      title="Alerting"
      description="Your Felukka's outward communications: pushes, builds and deployments, signalled to Slack, Teams and webhooks."
      actions={
        <button
          type="button"
          className="mz-glow"
          onClick={() => setCreating(true)}
        >
          <Icon name="add_circle" size={20} /> New alert
        </button>
      }
    >
      {loading && <Progress />}
      {error && (
        <div className="mz-alert">
          <strong>Could not load alerts.</strong> {error.message}
        </div>
      )}
      {value && (
        <div className="mz-bento">
          <div>
            <Section icon={<Icon name="sensors" />} title="Alerts">
              {value.alerts.length ? (
                <List alerts={value.alerts} />
              ) : (
                <span className="mz-muted">
                  No alerts yet. Create one to hear about pushes, builds and
                  deployments.
                </span>
              )}
            </Section>
            <Section icon={<Icon name="history" />} title="Recent signals">
              {value.deliveries.length ? (
                <Feed deliveries={value.deliveries} />
              ) : (
                <span className="mz-muted">Nothing sent yet.</span>
              )}
            </Section>
          </div>
          <div>
            <Section title="Signal status">
              <div style={{ display: 'grid', gap: 12 }}>
                <Row label="Alerts" value={value.alerts.length} />
                <Row
                  label="Channels"
                  value={value.alerts.reduce(
                    (n, a) => n + a.spec.channels.length,
                    0,
                  )}
                />
                <Row label="Recently delivered" value={delivered} />
                <Row label="Recently failed" value={failed} />
              </div>
            </Section>
            <div
              className="mz-panel"
              style={{ padding: 0, overflow: 'hidden' }}
            >
              <img
                src={mast}
                alt="Signal mast at dusk"
                style={{ width: '100%', display: 'block', opacity: 0.8 }}
              />
            </div>
          </div>
        </div>
      )}
      {creating && (
        <New
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            retry();
          }}
        />
      )}
    </KoptanPage>
  );
};
