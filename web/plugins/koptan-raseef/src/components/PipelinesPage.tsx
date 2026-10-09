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

/** In-cluster address of the Service the operator creates for a CD (port 80). */
const endpoint = (p: Pipeline) =>
  p.cd
    ? `http://${p.cd.metadata.name}.${p.cd.metadata.namespace ?? 'default'}.svc.cluster.local`
    : undefined;

const progress = (phase?: string) =>
  phase === 'Succeeded'
    ? 100
    : phase === 'Building' || phase === 'Resolving'
      ? 50
      : undefined;

/** Where the Dockerfile came from, and the stack it was generated for. */
const DockerfileOrigin = ({ service }: { service: Pipeline['service'] }) => {
  const d = service.status?.detected;
  if (service.status?.dockerfileSource !== 'template') {
    return <Badge>From repository</Badge>;
  }
  const stack = [d?.language, d?.version, d?.framework ?? d?.packageManager]
    .filter(Boolean)
    .join(' · ');
  return (
    <span title={d?.entrypoint ? `Starts ${d.entrypoint}` : undefined}>
      <Badge variant="primary">Generated</Badge>{' '}
      <span className="mz-code">{stack}</span>
    </span>
  );
};

const OriginCard = ({ service }: { service: Pipeline['service'] }) => (
  <div className="mz-card">
    <div className="mz-card-title">
      <span style={{ color: 'var(--mz-secondary)' }}>
        <Icon name="code" />
      </span>
      <span className="mz-label">Service Origin</span>
    </div>
    <div className="mz-card-corner">
      <Phase phase={service.status?.phase} />
    </div>
    <Field label="Source Repository">{service.spec.source.repo}</Field>
    <Field label="Active Revision">
      <span className="mz-code">{service.spec.source.revision ?? 'main'}</span>
    </Field>
    {service.status?.dockerfileSource && (
      <Field label="Dockerfile">
        <DockerfileOrigin service={service} />
      </Field>
    )}
    {service.status?.latestRevision && (
      <Field label="Latest Commit">
        <span className="mz-code">
          {service.status.latestRevision.slice(0, 12)}
        </span>
      </Field>
    )}
    {(service.status?.error ||
      (service.status?.phase === 'Failed' && service.status.message)) && (
      <div className="mz-error-text">
        {service.status?.error ?? service.status?.message}
      </div>
    )}
  </div>
);

const CICard = ({ ci }: { ci?: Pipeline['ci'] }) => (
  <div className={`mz-card${ci ? '' : ' mz-card--faded'}`}>
    <div className="mz-card-title">
      <span style={{ color: 'var(--mz-secondary)' }}>
        <Icon name="precision_manufacturing" />
      </span>
      <span className="mz-label">CI Build</span>
    </div>
    {ci ? (
      <>
        <div className="mz-card-corner">
          <Phase phase={ci.status?.phase} />
        </div>
        <Field label="Pipeline ID">{ci.metadata.name}</Field>
        <Field label="Target">
          {ci.spec.image.registry}/{ci.spec.image.repo}
        </Field>
        <Field label="Last Image">{ci.status?.latestImage ?? '—'}</Field>
        <Field label="Builds">{ci.status?.buildCount ?? 0}</Field>
        {!!ci.status?.pluginResults?.length && (
          <Field label="Plugin steps">
            {ci.status.pluginResults.map((r) => (
              <div key={r.name} title={r.message}>
                <Phase phase={r.phase} /> {r.name}
              </div>
            ))}
          </Field>
        )}
        {progress(ci.status?.phase) !== undefined && (
          <div className="mz-bar">
            <div style={{ width: `${progress(ci.status?.phase)}%` }} />
          </div>
        )}
        {ci.status?.message && (
          <div
            className={
              ci.status.phase === 'Failed'
                ? 'mz-error-text'
                : 'mz-muted mz-label'
            }
          >
            {ci.status.message}
          </div>
        )}
      </>
    ) : (
      <span className="mz-muted">No CI yet</span>
    )}
  </div>
);

const CDCard = ({ pipeline }: { pipeline: Pipeline }) => {
  const { cd } = pipeline;
  const phase = cd?.status?.phase;
  const active = phaseTone(phase) === 'ok';
  const target = cd?.spec.replicas ?? 1;
  const ready = cd?.status?.availableReplicas ?? 0;
  return (
    <div
      className={`mz-card${cd ? (active ? ' mz-card--active' : '') : ' mz-card--faded'}`}
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
          CD Deploy
        </span>
      </div>
      {cd ? (
        <>
          <div className="mz-card-corner">
            <Phase phase={phase} />
          </div>
          <Field label="Endpoint">
            <span style={{ color: 'var(--mz-tertiary)' }}>
              {endpoint(pipeline)}
            </span>
          </Field>
          <Field label={`Replicas: ${ready} ready of ${target}`}>
            <div className="mz-replicas">
              {Array.from({ length: target }, (_, i) => (
                <div
                  // biome-ignore lint/suspicious/noArrayIndexKey: replica slots are positional
                  key={i}
                  className={`mz-replica${i < ready ? ' mz-replica--on' : ''}`}
                />
              ))}
            </div>
          </Field>
          <Field label="Image">
            {cd.status?.latestImage ?? 'Not deployed'}
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
        <span className="mz-muted">No CD yet</span>
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
  const { service, cd } = pipeline;
  return (
    <div className="mz-pipeline">
      <div className="mz-pipeline-head">
        <div className="mz-pipeline-name">
          <span className="mz-muted">{String(index + 1).padStart(2, '0')}</span>
          {service.metadata.name}
          {service.status?.serviceType && (
            <Badge variant={kindVariant(service.status.serviceType)}>
              {service.status.serviceType}
            </Badge>
          )}
        </div>
        <div className="mz-label mz-label--dim">
          {service.metadata.namespace} / {cd?.metadata.name ?? 'no cd'}
        </div>
      </div>
      <div className="mz-flow">
        <OriginCard service={service} />
        <Arrow />
        <CICard ci={pipeline.ci} />
        <Arrow />
        <CDCard pipeline={pipeline} />
      </div>
    </div>
  );
};

/** Loads the pipelines and renders them as Service -> CI -> CD rows. */
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
      <span className="mz-muted">No Koptan services found in the cluster.</span>
    );
  }
  return (
    <>
      {value.map((p, i) => (
        <PipelineRow
          key={[
            p.service.metadata.namespace,
            p.service.metadata.name,
            p.ci?.metadata.name,
            p.cd?.metadata.name,
          ].join('/')}
          pipeline={p}
          index={i}
        />
      ))}
    </>
  );
};
