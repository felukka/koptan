import type { PluginRun } from '@internal/plugin-koptan-common';
import { Phase } from '@internal/plugin-koptan-react';

/** The plugin steps of each Service's latest build. */
export const RunsTable = ({ runs }: { runs: PluginRun[] }) => (
  <table className="mz-table">
    <thead>
      <tr>
        <th>Service</th>
        <th>Revision</th>
        <th>Build</th>
        <th>Steps</th>
      </tr>
    </thead>
    <tbody>
      {runs.map((run) => (
        <tr key={`${run.namespace}/${run.ci}`}>
          <td style={{ fontWeight: 700 }}>
            {run.service}
            <div className="mz-muted mz-label">{run.namespace}</div>
          </td>
          <td className="mz-code">{run.revision?.slice(0, 12) ?? '—'}</td>
          <td>
            <Phase phase={run.phase} />
          </td>
          <td>
            {run.results.map((r) => (
              <div key={r.name} title={r.message} style={{ marginBottom: 4 }}>
                <Phase phase={r.phase} /> <strong>{r.name}</strong>
                {r.message && (
                  <span className="mz-muted"> — {r.message.slice(-160)}</span>
                )}
              </div>
            ))}
          </td>
        </tr>
      ))}
    </tbody>
  </table>
);
