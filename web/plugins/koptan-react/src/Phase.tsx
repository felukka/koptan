const TONE: Record<string, 'ok' | 'warn' | 'bad'> = {
  Ready: 'ok',
  Succeeded: 'ok',
  Running: 'ok',
  Idle: 'warn',
  Pending: 'warn',
  Discovering: 'warn',
  Resolving: 'warn',
  Building: 'warn',
  Waiting: 'warn',
  Deploying: 'warn',
  Failed: 'bad',
};

/** The colour class for a phase name. */
export const phaseTone = (phase?: string) =>
  (phase && TONE[phase]) || ('warn' as const);

/** A phase name with a status dot, coloured by health. */
export const Phase = ({ phase }: { phase?: string }) => {
  const tone = phaseTone(phase);
  return (
    <span className={`mz-status mz-status--${tone}`}>
      <span className={`mz-dot mz-dot--${tone}`} />
      {phase ?? 'Unknown'}
    </span>
  );
};

type Staged = { status?: { phase?: string } } | undefined;

/**
 * One status for a Service -> CI -> CD chain: Failed if any stage failed,
 * Running once the CD runs, otherwise the first stage that is not done.
 */
export const pipelinePhase = (p: {
  service: Staged;
  ci?: Staged;
  cd?: Staged;
}): string => {
  const s = p.service?.status?.phase;
  const c = p.ci?.status?.phase;
  const d = p.cd?.status?.phase;
  if ([s, c, d].includes('Failed')) return 'Failed';
  if (s !== 'Ready') return s ?? 'Pending';
  if (d === 'Running') return 'Running';
  if (c !== 'Succeeded') return c ?? 'Waiting';
  return d ?? 'Waiting';
};
