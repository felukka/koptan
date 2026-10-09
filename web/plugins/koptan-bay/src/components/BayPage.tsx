import { Progress } from '@backstage/core-components';
import type {
  ActivityEntry,
  ClusterInfo,
  Overview,
  Pipeline,
} from '@internal/plugin-koptan-common';
import {
  Badge,
  Icon,
  KoptanPage,
  Phase,
  pipelinePhase,
  Section,
  kindVariant,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { Link } from 'react-router-dom';
import useAsync from 'react-use/esm/useAsync';

const Metric = ({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) => (
  <div>
    <div className="mz-metric-label">{label}</div>
    <div className="mz-metric-value">{children}</div>
  </div>
);

const Dim = ({ children }: { children: React.ReactNode }) => (
  <span className="mz-muted" style={{ fontSize: 10 }}>
    {children}
  </span>
);

const when = (iso: string) => {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  const time = d.toLocaleTimeString([], { hour12: false });
  return d.toDateString() === new Date().toDateString()
    ? time
    : `${d.toLocaleDateString([], { month: 'short', day: 'numeric' })} ${time}`;
};

const MetricsBar = ({
  overview,
  cluster,
}: {
  overview: Overview;
  cluster?: ClusterInfo;
}) => {
  const running = overview.cds.byPhase.Running ?? 0;
  return (
    <section className="mz-panel mz-panel--gold">
      <div className="mz-metrics">
        <div className="mz-panel-title">
          <span style={{ color: 'var(--mz-primary)' }}>
            <Icon name="query_stats" filled />
          </span>
          <span className="mz-label">Core Metrics</span>
        </div>
        <div className="mz-divider" />
        <Metric label="Node Count">
          {cluster && !cluster.error ? cluster.nodes.ready : '—'}
          <Dim>/ {cluster && !cluster.error ? cluster.nodes.total : '—'}</Dim>
        </Metric>
        <Metric label="Builds Running">
          <span
            className={`mz-dot ${overview.cis.byPhase.Building ? 'mz-dot--warn' : 'mz-dot--ok'}`}
          />
          {overview.cis.byPhase.Building ?? 0}
          <Dim>/ {overview.cis.total}</Dim>
        </Metric>
        <Metric label="Kube API Version">
          <span
            className={`mz-dot ${cluster?.kubernetesVersion ? 'mz-dot--ok' : 'mz-dot--warn'}`}
          />
          {cluster?.kubernetesVersion ?? '—'}
        </Metric>
        <Metric label="Active Deployments">
          <span className="mz-dot mz-dot--ok" />
          {running}
          <Dim>/ {overview.cds.total}</Dim>
        </Metric>
      </div>
      {cluster?.error && (
        <div className="mz-error-text">
          Cluster details unavailable: {cluster.error}
        </div>
      )}
    </section>
  );
};

const Deployments = ({ pipelines }: { pipelines: Pipeline[] }) => (
  <Section icon={<Icon name="deployed_code" />} title="Managed Deployments">
    <div style={{ overflowX: 'auto' }}>
      <table className="mz-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Language</th>
            <th>Ready / Replicas</th>
            <th>Status</th>
            <th className="mz-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {pipelines.length === 0 && (
            <tr>
              <td colSpan={5} className="mz-muted">
                No Koptan services found in the cluster.
              </td>
            </tr>
          )}
          {pipelines.map(({ service, ci, cd }) => (
            <tr
              key={[
                service.metadata.namespace,
                service.metadata.name,
                ci?.metadata.name,
                cd?.metadata.name,
              ].join('/')}
            >
              <td>
                <div style={{ fontWeight: 700 }}>{service.metadata.name}</div>
                <div className="mz-label mz-label--dim">
                  ID: {cd?.metadata.name ?? service.metadata.name}
                </div>
              </td>
              <td>
                {service.status?.serviceType ? (
                  <Badge variant={kindVariant(service.status.serviceType)}>
                    {service.status.serviceType}
                  </Badge>
                ) : (
                  <span className="mz-muted">detecting…</span>
                )}
              </td>
              <td className="mz-mono">
                {cd
                  ? `${cd.status?.availableReplicas ?? 0} / ${cd.spec.replicas ?? 1}`
                  : '—'}
              </td>
              <td>
                <Phase phase={pipelinePhase({ service, ci, cd })} />
              </td>
              <td className="mz-right">
                <Link to="/raseef" className="mz-link-btn">
                  Open pipeline
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  </Section>
);

const ActivityLog = ({ entries }: { entries: ActivityEntry[] }) => (
  <Section icon={<Icon name="history_edu" />} title="Recent Activity Log">
    {entries.length === 0 && <span className="mz-muted">Nothing yet.</span>}
    {entries.map((e) => (
      <div
        className="mz-log-row"
        key={`${e.time}/${e.kind}/${e.namespace}/${e.name}`}
      >
        <div className="mz-log-time">{when(e.time)}</div>
        <span
          className={`mz-dot ${e.severity === 'error' ? 'mz-dot--bad' : e.severity === 'success' ? 'mz-dot--ok' : 'mz-dot--warn'}`}
        />
        <div style={{ flex: 1 }}>
          <div className="mz-log-msg">{e.message}</div>
          <div className="mz-log-sub">
            {e.kind}: {e.namespace}/{e.name}
          </div>
        </div>
      </div>
    ))}
  </Section>
);

/** "The Bay": central command for services, builds and deployments. */
export const BayPage = () => {
  const { getOverview, getPipelines, getActivity, getCluster } = useKoptanApi();
  const { value, loading, error } = useAsync(async () => {
    const [overview, pipelines, activity] = await Promise.all([
      getOverview(),
      getPipelines(),
      getActivity(),
    ]);
    // Cluster facts are best effort; the page works without them.
    const cluster = await getCluster().catch(
      (e): ClusterInfo => ({
        nodes: { total: 0, ready: 0 },
        error: (e as Error).message,
      }),
    );
    return { overview, pipelines, activity, cluster };
  }, [getOverview, getPipelines, getActivity, getCluster]);

  const description = (
    <>
      Central command and telemetry orchestration.
      <br />
      Current state:{' '}
      <span className="mz-accent">
        {error
          ? 'Cluster unreachable.'
          : value?.pipelines.some((p) => pipelinePhase(p) === 'Failed')
            ? 'Rough seas, something failed.'
            : 'Steady as she goes.'}
      </span>
    </>
  );

  return (
    <KoptanPage title="The Bay" description={description}>
      {loading && <Progress />}
      {error && (
        <div className="mz-alert">
          <strong>Could not load Koptan.</strong> {error.message}
        </div>
      )}
      {value && (
        <>
          <MetricsBar overview={value.overview} cluster={value.cluster} />
          <Deployments pipelines={value.pipelines} />
          <ActivityLog entries={value.activity} />
        </>
      )}
    </KoptanPage>
  );
};
