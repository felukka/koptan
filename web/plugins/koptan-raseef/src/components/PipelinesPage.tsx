import { Progress } from '@backstage/core-components';
import type { Pipeline } from '@internal/plugin-koptan-common';
import {
  Badge,
  Icon,
  kindVariant,
  Phase,
  phaseTone,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import useAsyncRetry from 'react-use/esm/useAsyncRetry';

const Field = ({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) => (
  <div className="mz-field">
    <div className="mz-label mz-label--dim">{label}</div>
    <div className="mz-field-value">{children}</div>
  </div>
);

const Arrow = () => (
  <div className="mz-arrow">
    <Icon name="arrow_forward" size={18} />
  </div>
);

/** In-cluster address of the Service the operator creates for a voyage. */
const endpoint = (p: Pipeline) =>
  p.voyage
    ? `http://${p.voyage.metadata.name}.${p.voyage.metadata.namespace ?? 'default'}.svc.cluster.local:${p.voyage.spec.port}`
    : undefined;

const progress = (phase?: string) =>
  phase === 'Succeeded'
    ? 100
    : phase === 'Building' || phase === 'Resolving'
      ? 50
      : undefined;

const OriginCard = ({ app }: { app: Pipeline['app'] }) => (
  <div className="mz-card">
    <div className="mz-card-title">
      <span style={{ color: 'var(--mz-secondary)' }}>
        <Icon name="code" />
      </span>
      <span className="mz-label">App Origin</span>
    </div>
    <div className="mz-card-corner">
      <Phase phase={app.status?.phase} />
    </div>
    <Field label="Source Repository">{app.spec.source.repo}</Field>
    <Field label="Active Revision">
      <span className="mz-code">{app.spec.source.revision ?? 'main'}</span>
    </Field>
    {app.status?.error && (
      <div className="mz-error-text">{app.status.error}</div>
    )}
  </div>
);

const SlipwayCard = ({ slipway }: { slipway?: Pipeline['slipway'] }) => (
  <div className={`mz-card${slipway ? '' : ' mz-card--faded'}`}>
    <div className="mz-card-title">
      <span style={{ color: 'var(--mz-secondary)' }}>
        <Icon name="precision_manufacturing" />
      </span>
      <span className="mz-label">Slipway CI</span>
    </div>
    {slipway ? (
      <>
        <div className="mz-card-corner">
          <Phase phase={slipway.status?.phase} />
        </div>
        <Field label="Pipeline ID">{slipway.metadata.name}</Field>
        <Field label="Artifact Path">
          {slipway.status?.latestImage ?? '—'}
        </Field>
        <Field label="Builds">{slipway.status?.buildCount ?? 0}</Field>
        {progress(slipway.status?.phase) !== undefined && (
          <div className="mz-bar">
            <div style={{ width: `${progress(slipway.status?.phase)}%` }} />
          </div>
        )}
        {slipway.status?.message && (
          <div
            className={
              slipway.status.phase === 'Failed'
                ? 'mz-error-text'
                : 'mz-muted mz-label'
            }
          >
            {slipway.status.message}
          </div>
        )}
      </>
    ) : (
      <span className="mz-muted">No slipway</span>
    )}
  </div>
);

const VoyageCard = ({ pipeline }: { pipeline: Pipeline }) => {
  const { voyage } = pipeline;
  const phase = voyage?.status?.phase;
  const active = phaseTone(phase) === 'ok';
  const target = voyage?.spec.replicas ?? 1;
  return (
    <div
      className={`mz-card${voyage ? (active ? ' mz-card--active' : '') : ' mz-card--faded'}`}
    >
      <div className="mz-card-title">
        <span
          style={{
            color: active ? 'var(--mz-primary)' : 'var(--mz-secondary)',
          }}
        >
          <Icon name="rocket_launch" filled={active} />
        </span>
        <span
          className="mz-label"
          style={{ color: active ? 'var(--mz-primary)' : undefined }}
        >
          Voyage CD
        </span>
      </div>
      {voyage ? (
        <>
          <div className="mz-card-corner">
            <Phase phase={phase} />
          </div>
          <Field label="Endpoint">
            <span style={{ color: 'var(--mz-tertiary)' }}>
              {endpoint(pipeline)}
            </span>
          </Field>
          <Field label="Target Replicas">
            <div className="mz-replicas">
              {Array.from({ length: target }, (_, i) => (
                <div
                  // biome-ignore lint/suspicious/noArrayIndexKey: replica slots are positional
                  key={i}
                  className={`mz-replica${phase === 'Running' ? ' mz-replica--on' : ''}`}
                />
              ))}
            </div>
          </Field>
          <Field label="Image">
            {voyage.status?.deployedImage ?? 'Not deployed'}
          </Field>
          <div className="mz-actions">
            <button
              type="button"
              className="mz-btn mz-btn--primary"
              disabled
              title="Manual scaling is not wired up yet"
            >
              Manual Scale
            </button>
            <button
              type="button"
              className="mz-btn"
              disabled
              title="Log streaming is not wired up yet"
            >
              <Icon name="terminal" size={16} /> View Logs
            </button>
          </div>
        </>
      ) : (
        <span className="mz-muted">No voyage</span>
      )}
    </div>
  );
};

const PipelineRow = ({
  pipeline,
  index,
}: {
  pipeline: Pipeline;
  index: number;
}) => {
  const { app, voyage } = pipeline;
  return (
    <div className="mz-pipeline">
      <div className="mz-pipeline-head">
        <div className="mz-pipeline-name">
          <span className="mz-muted">{String(index + 1).padStart(2, '0')}</span>
          {app.metadata.name}
          <Badge variant={kindVariant(app.kind)}>{app.kind}</Badge>
        </div>
        <div className="mz-label mz-label--dim">
          {app.metadata.namespace} / {voyage?.metadata.name ?? 'no voyage'}
        </div>
      </div>
      <div className="mz-flow">
        <OriginCard app={app} />
        <Arrow />
        <SlipwayCard slipway={pipeline.slipway} />
        <Arrow />
        <VoyageCard pipeline={pipeline} />
      </div>
    </div>
  );
};

/** Loads the pipelines and renders them as App -> Slipway -> Voyage rows. */
export const usePipelines = () => {
  const { getPipelines } = useKoptanApi();
  return useAsyncRetry(getPipelines, [getPipelines]);
};

export const PipelinesList = ({
  value,
  loading,
  error,
}: ReturnType<typeof usePipelines>) => {
  if (loading) return <Progress />;
  if (error) {
    return (
      <div className="mz-alert">
        <strong>Could not load pipelines.</strong> {error.message}
      </div>
    );
  }
  if (!value?.length) {
    return (
      <span className="mz-muted">No Koptan apps found in the cluster.</span>
    );
  }
  return (
    <>
      {value.map((p, i) => (
        <PipelineRow
          key={[
            p.app.metadata.namespace,
            p.app.metadata.name,
            p.slipway?.metadata.name,
            p.voyage?.metadata.name,
          ].join('/')}
          pipeline={p}
          index={i}
        />
      ))}
    </>
  );
};
