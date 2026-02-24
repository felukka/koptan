import type {
  ActivityEntry,
  CD,
  CI,
  Service,
} from '@internal/plugin-koptan-common';

type Stamped = {
  kind?: string;
  metadata: { name: string; namespace?: string; creationTimestamp?: string };
  status?: {
    phase?: string;
    error?: string;
    message?: string;
    latestImage?: string;
    lastBuildTime?: string;
    lastPushDetected?: string;
    conditions?: { lastTransitionTime?: string }[];
  };
};

/** Newest timestamp on a resource: last build or push, a condition, or creation. */
function lastTouched(r: Stamped): string {
  const times = [
    r.status?.lastBuildTime,
    r.status?.lastPushDetected,
    ...(r.status?.conditions ?? []).map((c) => c.lastTransitionTime),
    r.metadata.creationTimestamp,
  ].filter((t): t is string => !!t);
  return times.sort().at(-1) ?? '';
}

function describe(
  kind: ActivityEntry['kind'],
  r: Stamped,
): Pick<ActivityEntry, 'message' | 'severity'> {
  const phase = r.status?.phase;
  const name = r.metadata.name;
  if (phase === 'Failed') {
    const why = r.status?.error ?? r.status?.message;
    return {
      message: `${kind} ${name} failed${why ? `: ${why}` : ''}`,
      severity: 'error',
    };
  }
  if (kind === 'Service') {
    return phase === 'Ready'
      ? { message: `Service ${name} is ready`, severity: 'success' }
      : { message: `Service ${name} is ${phase ?? 'new'}`, severity: 'info' };
  }
  if (kind === 'CI') {
    return phase === 'Succeeded'
      ? {
          message: `Build completed for ${name}${
            r.status?.latestImage ? ` (${r.status.latestImage})` : ''
          }`,
          severity: 'success',
        }
      : { message: `Build ${name} is ${phase ?? 'new'}`, severity: 'info' };
  }
  return phase === 'Running'
    ? {
        message: `Deployment ${name} is running${
          r.status?.latestImage ? ` ${r.status.latestImage}` : ''
        }`,
        severity: 'success',
      }
    : { message: `Deployment ${name} is ${phase ?? 'new'}`, severity: 'info' };
}

/** A recent-activity feed derived from the current state of each resource. */
export function buildActivity(
  services: Service[],
  cis: CI[],
  cds: CD[],
  limit = 20,
): ActivityEntry[] {
  const entries = (
    [
      ['Service', services],
      ['CI', cis],
      ['CD', cds],
    ] as const
  ).flatMap(([kind, items]) =>
    (items as Stamped[]).map((r) => ({
      time: lastTouched(r),
      kind,
      name: r.metadata.name,
      namespace: r.metadata.namespace,
      ...describe(kind, r),
    })),
  );
  return entries.sort((a, b) => b.time.localeCompare(a.time)).slice(0, limit);
}
