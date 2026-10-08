import type {
  ActivityEntry,
  CD,
  CI,
  CreatePipelineRequest,
  Overview,
  Pipeline,
  Service,
} from '@internal/plugin-koptan-common';
import { ConflictError, NotAllowedError } from '@backstage/errors';
import type { KoptanClient, RawResource } from './crdClient';

const LAST_APPLIED = 'kubectl.kubernetes.io/last-applied-configuration';

/**
 * `kubectl apply` stores a full copy of the spec (tokens included) in the
 * last-applied annotation, and managedFields is noise, so drop both.
 */
function cleanMetadata(raw: RawResource): RawResource {
  const { managedFields: _mf, annotations, ...meta } = raw.metadata ?? {};
  const { [LAST_APPLIED]: _la, ...rest } = annotations ?? {};
  return Object.keys(rest).length ? { ...meta, annotations: rest } : meta;
}

/** The secretRef only names a Secret, so nothing to strip beyond noise. */
export function redactService(raw: RawResource): Service {
  return { ...raw, metadata: cleanMetadata(raw) } as Service;
}

/** Drops the registry login so credentials never reach the browser. */
export function redactCI(raw: RawResource): CI {
  const { loginSecret: _omit, ...image } = raw.spec?.image ?? {};
  return {
    ...raw,
    metadata: cleanMetadata(raw),
    spec: { ...raw.spec, image },
  } as CI;
}

export function redactCD(raw: RawResource): CD {
  return { ...raw, metadata: cleanMetadata(raw) } as CD;
}

const key = (ns: string | undefined, name: string) => `${ns ?? ''}/${name}`;

/**
 * Joins Service -> CI (spec.service.name) -> CD (spec.ci.name).
 * A service can have several CIs and a CI several CDs, so this emits one
 * pipeline per chain; a service or CI with nothing downstream still gets one.
 */
export function buildPipelines(
  services: Service[],
  cis: CI[],
  cds: CD[],
): Pipeline[] {
  const group = <T>(items: T[], k: (item: T) => string) => {
    const m = new Map<string, T[]>();
    for (const i of items) m.set(k(i), [...(m.get(k(i)) ?? []), i]);
    return m;
  };
  const cisByService = group(cis, (c) =>
    key(c.metadata.namespace, c.spec.service.name),
  );
  const cdsByCI = group(cds, (d) => key(d.metadata.namespace, d.spec.ci.name));
  return services.flatMap((service) => {
    const mine =
      cisByService.get(
        key(service.metadata.namespace, service.metadata.name),
      ) ?? [];
    if (mine.length === 0) return [{ service }];
    return mine.flatMap((ci) => {
      const ds =
        cdsByCI.get(key(ci.metadata.namespace, ci.metadata.name)) ?? [];
      return ds.length === 0
        ? [{ service, ci }]
        : ds.map((cd) => ({ service, ci, cd }));
    });
  });
}

function countByPhase(items: { status?: { phase?: string } }[]) {
  const out: Record<string, number> = {};
  for (const i of items) {
    const phase = i.status?.phase ?? 'Unknown';
    out[phase] = (out[phase] ?? 0) + 1;
  }
  return out;
}

export function buildOverview(
  services: Service[],
  cis: CI[],
  cds: CD[],
): Overview {
  return {
    services: { total: services.length, byPhase: countByPhase(services) },
    cis: { total: cis.length, byPhase: countByPhase(cis) },
    cds: { total: cds.length, byPhase: countByPhase(cds) },
    replicas: {
      desired: cds.reduce((n, d) => n + (d.spec.replicas ?? 1), 0),
    },
  };
}

export async function loadAll(client: KoptanClient, namespace?: string) {
  const [services, cis, cds] = await Promise.all([
    client.list('Service', namespace),
    client.list('CI', namespace),
    client.list('CD', namespace),
  ]);
  return {
    services: services.map(redactService),
    cis: cis.map(redactCI),
    cds: cds.map(redactCD),
  };
}

const DNS_LABEL = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;

export function validateCreate(req: CreatePipelineRequest): string | undefined {
  if (!DNS_LABEL.test(req.name ?? '')) return 'name must be a DNS-1123 label';
  if (!req.repo) return 'repo is required';
  for (const e of req.env ?? []) {
    if (!e.name) return 'env variables need a name';
  }
  return undefined;
}

/** Creates the Service; the operator derives its CI and CD. */
export async function createPipeline(
  client: KoptanClient,
  req: CreatePipelineRequest,
  createdBy: string,
): Promise<Pipeline> {
  const namespace = req.namespace ?? 'default';
  // The operator reads the git token from a Secret, so store it as one
  // instead of putting it in the spec.
  if (req.token) {
    await client.createSecret(namespace, `${req.name}-git`, {
      token: req.token,
    });
  }
  const service = await client.create('Service', namespace, {
    metadata: {
      name: req.name,
      namespace,
      annotations: { 'koptan.felukka.org/created-by': createdBy },
    },
    spec: {
      source: {
        repo: req.repo,
        ...(req.revision ? { revision: req.revision } : {}),
        ...(req.token
          ? { secretRef: { name: `${req.name}-git`, key: 'token' } }
          : {}),
      },
      ...(req.env?.length ? { env: req.env } : {}),
    },
  });
  return { service: redactService({ kind: 'Service', ...service }) };
}

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

/** Turns Kubernetes API failures into short, readable errors. */
export function friendlyError(e: unknown, req: CreatePipelineRequest): unknown {
  const code = (e as { code?: number }).code;
  const ns = req.namespace ?? 'default';
  if (code === 409) {
    return new ConflictError(
      `"${req.name}" already exists in namespace "${ns}" (a service or secret with that name)`,
    );
  }
  if (code === 403) {
    return new NotAllowedError(
      'The service account used by Backstage is not allowed to create these resources; see config/rbac/backstage_role.yaml',
    );
  }
  return e;
}
