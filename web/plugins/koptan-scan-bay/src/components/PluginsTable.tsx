import type { CIPlugin } from '@internal/plugin-koptan-common';
import { Badge, type BadgeVariant } from '@internal/plugin-koptan-react';

const TYPE_VARIANT: Record<CIPlugin['spec']['type'], BadgeVariant> = {
  sonarqube: 'primary',
  codeql: 'success',
  custom: 'default',
};

/** What the plugin runs, in one line. */
const describe = (p: CIPlugin) => {
  const s = p.spec;
  if (s.sonarqube) return s.sonarqube.hostURL;
  if (s.codeql)
    return `${s.codeql.querySuite ?? 'code-scanning'}, fails on ${s.codeql.failOnSeverity ?? 'error'}`;
  if (s.custom) return [s.custom.image, ...(s.custom.command ?? [])].join(' ');
  return '';
};

/** How the plugin attaches itself, besides Services listing it. */
const attachment = (p: CIPlugin) => {
  const refs = p.spec.targetRefs?.map((r) => r.name) ?? [];
  const labels = Object.entries(p.spec.selector?.matchLabels ?? {}).map(
    ([k, v]) => `${k}=${v}`,
  );
  return [...refs, ...labels].join(', ') || 'by Service';
};

const accepted = (p: CIPlugin) =>
  p.status?.conditions?.find((c) => c.type === 'Accepted');

export const PluginsTable = ({ plugins }: { plugins: CIPlugin[] }) => (
  <table className="mz-table">
    <thead>
      <tr>
        <th>Plugin</th>
        <th>Type</th>
        <th>Runs</th>
        <th>Order</th>
        <th>Attaches</th>
        <th>Services</th>
        <th>Status</th>
      </tr>
    </thead>
    <tbody>
      {plugins.map((p) => {
        const cond = accepted(p);
        const ok = cond?.status === 'True';
        return (
          <tr key={`${p.metadata.namespace}/${p.metadata.name}`}>
            <td style={{ fontWeight: 700 }}>{p.metadata.name}</td>
            <td>
              <Badge variant={TYPE_VARIANT[p.spec.type]}>{p.spec.type}</Badge>
            </td>
            <td className="mz-muted mz-code">{describe(p)}</td>
            <td>
              {p.spec.order ?? 100}
              {p.spec.failurePolicy === 'Ignore' && (
                <span className="mz-muted"> · ignore failures</span>
              )}
            </td>
            <td className="mz-muted">{attachment(p)}</td>
            <td>{p.status?.attachedServices?.join(', ') || '—'}</td>
            <td
              className={`mz-status mz-status--${ok ? 'ok' : cond ? 'bad' : 'warn'}`}
              title={cond?.message}
            >
              {ok ? 'Accepted' : cond ? 'Invalid' : 'Pending'}
            </td>
          </tr>
        );
      })}
    </tbody>
  </table>
);
