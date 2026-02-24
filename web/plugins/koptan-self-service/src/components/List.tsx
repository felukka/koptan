import { Progress } from '@backstage/core-components';
import type { SelfService } from '@internal/plugin-koptan-common';
import {
  Badge,
  Icon,
  Phase,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { Link } from 'react-router-dom';
import useAsyncRetry from 'react-use/esm/useAsyncRetry';

export const useSelfServices = () => {
  const { getSelfServices } = useKoptanApi();
  return useAsyncRetry(getSelfServices, [getSelfServices]);
};

export const repoLabel = (s: SelfService) =>
  s.status?.repoURL ??
  s.spec.repo.existing?.url ??
  `${s.spec.repo.create?.provider}: ${s.spec.repo.create?.owner ?? '(token owner)'}/${s.spec.repo.create?.name ?? s.metadata.name}`;

export const SessionList = ({
  value,
  loading,
  error,
}: ReturnType<typeof useSelfServices>) => {
  if (loading) return <Progress />;
  if (error) {
    return (
      <div className="mz-alert">
        <strong>Could not load sessions.</strong> {error.message}
      </div>
    );
  }
  if (!value?.length) {
    return (
      <span className="mz-muted">
        No sessions yet. Start one to have an agent build a service for you.
      </span>
    );
  }
  return (
    <div className="mz-cards">
      {value.map((s) => (
        <Link
          key={`${s.metadata.namespace}/${s.metadata.name}`}
          to={`/self-service/${s.metadata.namespace ?? 'default'}/${s.metadata.name}`}
          className="mz-card mz-card--link"
        >
          <div className="mz-card-title">
            <span style={{ color: 'var(--mz-secondary)' }}>
              <Icon name="auto_awesome" />
            </span>
            <span className="mz-label">{s.metadata.name}</span>
          </div>
          <div className="mz-card-corner">
            <Phase phase={s.status?.phase} />
          </div>
          <div className="mz-code mz-muted" style={{ wordBreak: 'break-all' }}>
            {repoLabel(s)}
          </div>
          <div style={{ marginTop: 12, display: 'flex', gap: 6 }}>
            <Badge>{s.spec.ai.provider}</Badge>
            <Badge variant="primary">{s.spec.ai.model}</Badge>
            {s.spec.allowCommands && <Badge variant="warning">commands</Badge>}
          </div>
        </Link>
      ))}
    </div>
  );
};
