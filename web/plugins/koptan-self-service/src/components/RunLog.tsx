import type { AgentEvent, AgentRun } from '@internal/plugin-koptan-common';
import { Icon, Phase } from '@internal/plugin-koptan-react';

const ICON: Record<AgentEvent['type'], string> = {
  status: 'pending',
  message: 'chat',
  tool: 'build',
  tool_result: 'subdirectory_arrow_right',
  done: 'check_circle',
  error: 'error',
};

const line = (e: AgentEvent) => {
  switch (e.type) {
    case 'status':
    case 'message':
      return e.text;
    case 'tool':
      return `${e.name} ${JSON.stringify(e.input).slice(0, 160)}`;
    case 'tool_result':
      return e.output;
    case 'done':
      return e.commit
        ? `Pushed ${e.commit.slice(0, 12)}: ${e.summary}`
        : `No changes. ${e.summary}`;
    case 'error':
      return e.message;
    default:
      return '';
  }
};

export const EventLog = ({ events }: { events: AgentEvent[] }) => (
  <div className="mz-log" aria-live="polite">
    {events.map((e, i) => (
      <div
        // biome-ignore lint/suspicious/noArrayIndexKey: the log is append-only
        key={i}
        className={`mz-log-line mz-log-line--${e.type}${e.type === 'tool_result' && e.isError ? ' mz-log-line--error' : ''}`}
      >
        <Icon name={ICON[e.type]} size={16} />
        <span>{line(e)}</span>
      </div>
    ))}
  </div>
);

export const RunHistory = ({ runs }: { runs: AgentRun[] }) => (
  <table className="mz-table">
    <thead>
      <tr>
        <th>Prompt</th>
        <th>Result</th>
        <th>Commit</th>
        <th>Started</th>
      </tr>
    </thead>
    <tbody>
      {runs.map((r) => (
        <tr key={r.id}>
          <td style={{ maxWidth: 420 }}>{r.prompt}</td>
          <td title={r.error ?? r.summary}>
            <Phase
              phase={
                {
                  running: 'Building',
                  succeeded: 'Succeeded',
                  failed: 'Failed',
                }[r.status]
              }
            />
          </td>
          <td className={r.commit ? 'mz-code' : 'mz-muted'}>
            {r.commit ? r.commit.slice(0, 12) : '—'}
          </td>
          <td className="mz-muted">{new Date(r.startedAt).toLocaleString()}</td>
        </tr>
      ))}
    </tbody>
  </table>
);
