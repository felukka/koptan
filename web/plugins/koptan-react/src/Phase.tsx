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
