import {
  Badge,
  Dim,
  Icon,
  KoptanPage,
  SampleNote,
  Section,
} from '@internal/plugin-koptan-react';
import mast from '../signal-mast.png';

const CHANNELS = [
  {
    icon: 'tag',
    title: 'Slack: #ops-fleet-status',
    sub: 'Webhook ID: WH_SK_8829',
    state: 'Connected',
  },
  {
    icon: 'chat',
    title: 'Discord: minzar Monitor',
    sub: 'Integration active since 12d',
    state: 'Connected',
  },
  {
    icon: 'webhook',
    title: 'External API Gateway',
    sub: 'POST: https://telemetry.internal.io/v2',
    state: 'Paused',
  },
];

const RULES: { group: string; items: [string, string, boolean][] }[] = [
  {
    group: 'Cluster Events',
    items: [
      [
        'New Deployment',
        'Notify when a new image is pulled from the registry.',
        true,
      ],
    ],
  },
  {
    group: 'CI Builds',
    items: [
      ['Build Succeeded', 'CI pipeline completion status.', false],
      ['Build Failed', 'Immediate alert on build step failure.', true],
    ],
  },
  {
    group: 'CD Deployments',
    items: [
      ['Deploy Succeeded', 'Confirmation of rolling update success.', false],
      ['Deploy Failed', 'Rollback triggers or orchestration errors.', true],
    ],
  },
];

export const Page = () => (
  <KoptanPage
    title="The Signal Mast"
    description="Orchestrate your Felukka's outward communications. Configure real-time alerts and manage integration WebHooks."
  >
    <SampleNote>
      Alert channels are not wired up yet; the operator has no alerting backend.
      Everything below only shows the planned layout.
    </SampleNote>
    <div className="mz-bento">
      <div>
        <Section
          icon={<Icon name="sensors" />}
          title="Active Channels"
          actions={
            <button type="button" className="mz-btn" disabled>
              <Icon name="add" size={16} /> Add channel
            </button>
          }
        >
          {CHANNELS.map((c) => (
            <div className="mz-item" key={c.title}>
              <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
                <span style={{ color: 'var(--mz-primary)' }}>
                  <Icon name={c.icon} />
                </span>
                <div>
                  <div className="mz-item-title">{c.title}</div>
                  <div className="mz-label mz-label--dim">{c.sub}</div>
                </div>
              </div>
              <Badge variant={c.state === 'Connected' ? 'success' : 'warning'}>
                {c.state}
              </Badge>
            </div>
          ))}
        </Section>
        <Section icon={<Icon name="rule" />} title="Alert Logic">
          {RULES.map((g) => (
            <div key={g.group}>
              <div className="mz-subhead">{g.group}</div>
              {g.items.map(([title, text, on]) => (
                <div className="mz-item" key={title}>
                  <div>
                    <div className="mz-item-title">{title}</div>
                    <div className="mz-muted" style={{ fontSize: 12 }}>
                      {text}
                    </div>
                  </div>
                  <div className={`mz-toggle${on ? ' mz-toggle--on' : ''}`} />
                </div>
              ))}
            </div>
          ))}
        </Section>
      </div>
      <div>
        <Section title="Signal Status">
          <div style={{ display: 'grid', gap: 12 }}>
            <div className="mz-metric-value" style={{ fontSize: 28 }}>
              99.8% <Dim>Downtime: 2m</Dim>
            </div>
            <Row label="Signals Sent (24h)" value="12,402" />
            <Row label="Avg Latency" value="42ms" />
          </div>
        </Section>
        <div className="mz-panel" style={{ padding: 0, overflow: 'hidden' }}>
          <img
            src={mast}
            alt="Signal mast at dusk"
            style={{ width: '100%', display: 'block', opacity: 0.8 }}
          />
          <div style={{ padding: 20 }}>
            <div className="mz-item-title">Mast Calibration</div>
            <div className="mz-muted" style={{ fontSize: 12 }}>
              Last synced with celestial clock: 04:22 UTC
            </div>
          </div>
        </div>
      </div>
    </div>
  </KoptanPage>
);

const Row = ({ label, value }: { label: string; value: string }) => (
  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
    <span className="mz-muted">{label}</span>
    <span style={{ fontWeight: 700 }}>{value}</span>
  </div>
);
