import type { AgentEvent } from '@internal/plugin-koptan-common';
import {
  Icon,
  KoptanPage,
  Phase,
  pipelinePhase,
  Section,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { type FormEvent, useState } from 'react';
import { Link } from 'react-router-dom';
import useAsyncRetry from 'react-use/esm/useAsyncRetry';
import { EventLog, RunHistory } from './RunLog';
import { repoLabel } from './List';

export const Page = ({
  namespace,
  name,
}: {
  namespace: string;
  name: string;
}) => {
  const api = useKoptanApi();
  const session = useAsyncRetry(
    async () =>
      (await api.getSelfServices()).find(
        (s) =>
          s.metadata.name === name &&
          (s.metadata.namespace ?? 'default') === namespace,
      ),
    [api.getSelfServices, namespace, name],
  );
  const pipeline = useAsyncRetry(
    async () =>
      (await api.getPipelines()).find(
        (p) =>
          p.service.metadata.name === name &&
          p.service.metadata.namespace === namespace,
      ),
    [api.getPipelines, namespace, name],
  );
  const runs = useAsyncRetry(
    () => api.getAgentRuns(namespace, name),
    [api.getAgentRuns, namespace, name],
  );
  const [prompt, setPrompt] = useState('');
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [running, setRunning] = useState(false);

  const send = async (e: FormEvent) => {
    e.preventDefault();
    setRunning(true);
    setEvents([]);
    try {
      await api.runPrompt(namespace, name, prompt.trim(), (ev) =>
        setEvents((prev) => [...prev, ev]),
      );
      setPrompt('');
    } catch (err) {
      setEvents((prev) => [
        ...prev,
        { type: 'error', message: (err as Error).message },
      ]);
    } finally {
      setRunning(false);
      runs.retry();
      pipeline.retry();
    }
  };

  const s = session.value;
  const ready = s?.status?.phase === 'Ready' && !s.spec.suspend;
  return (
    <KoptanPage
      title={name}
      description={
        s ? (
          <span className="mz-code">{repoLabel(s)}</span>
        ) : (
          'Self Service session'
        )
      }
      actions={
        <Link to="/self-service" className="mz-btn">
          <Icon name="arrow_back" size={16} /> Sessions
        </Link>
      }
    >
      {session.error && <div className="mz-alert">{session.error.message}</div>}
      <div className="mz-bento">
        <div>
          <Section icon={<Icon name="auto_awesome" />} title="Ask the agent">
            <form onSubmit={send} style={{ display: 'grid', gap: 12 }}>
              <textarea
                className="mz-input mz-textarea"
                aria-label="Prompt"
                value={prompt}
                maxLength={8000}
                rows={5}
                placeholder="e.g. Create a FastAPI service with a /orders endpoint backed by an in-memory list"
                onChange={(e) => setPrompt(e.target.value)}
                disabled={running}
              />
              <div>
                <button
                  type="submit"
                  className="mz-glow"
                  disabled={running || !ready || !prompt.trim()}
                >
                  <Icon name="send" size={18} /> {running ? 'Working…' : 'Send'}
                </button>
                {!ready && s && (
                  <span className="mz-muted">
                    {' '}
                    {s.status?.message ?? 'The agent is not ready yet'}
                  </span>
                )}
              </div>
            </form>
            {events.length > 0 && <EventLog events={events} />}
          </Section>
          <Section icon={<Icon name="history" />} title="Runs">
            {runs.error ? (
              <span className="mz-muted">{runs.error.message}</span>
            ) : runs.value?.length ? (
              <RunHistory runs={runs.value} />
            ) : (
              <span className="mz-muted">No prompts yet.</span>
            )}
          </Section>
        </div>
        <div>
          <Section title="Session">
            <div style={{ display: 'grid', gap: 10 }}>
              <div>
                Agent: <Phase phase={s?.status?.phase} />
              </div>
              <div>
                Model:{' '}
                <span className="mz-code">
                  {s?.spec.ai.provider}/{s?.spec.ai.model}
                </span>
              </div>
              <div>
                Branch:{' '}
                <span className="mz-code">{s?.spec.branch ?? 'main'}</span>
              </div>
            </div>
          </Section>
          <Section title="Pipeline">
            {pipeline.value ? (
              <div style={{ display: 'grid', gap: 10 }}>
                <div>
                  Status: <Phase phase={pipelinePhase(pipeline.value)} />
                </div>
                <div className="mz-muted">
                  {pipeline.value.ci?.status?.message ??
                    pipeline.value.service.status?.message}
                </div>
                <Link to="/raseef" className="mz-link-btn">
                  Open in the Raseef
                </Link>
              </div>
            ) : (
              <span className="mz-muted">
                The Service appears once the repository is ready.
              </span>
            )}
          </Section>
        </div>
      </div>
    </KoptanPage>
  );
};
