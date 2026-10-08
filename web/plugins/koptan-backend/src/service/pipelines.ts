import {
  type ActivityEntry,
  APP_KINDS,
  type AppKind,
  type CreatePipelineRequest,
  type KoptanApp,
  type Overview,
  type Pipeline,
  type Slipway,
  type Voyage,
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

/** Removes the PAT so it is never sent to the browser. */
export function redactApp(raw: RawResource): KoptanApp {
  const { patToken: _omit, ...source } = raw.spec?.source ?? {};
  return {
    ...raw,
    metadata: cleanMetadata(raw),
    spec: { ...raw.spec, source },
  } as KoptanApp;
}

export function redactSlipway(raw: RawResource): Slipway {
  const { creds: _omit, ...image } = raw.spec?.image ?? {};
  return {
    ...raw,
    metadata: cleanMetadata(raw),
    spec: { ...raw.spec, image },
  } as Slipway;
}

export function redactVoyage(raw: RawResource): Voyage {
  return { ...raw, metadata: cleanMetadata(raw) } as Voyage;
}

const key = (ns: string | undefined, name: string) => `${ns ?? ''}/${name}`;

/**
 * Joins App -> Slipway (spec.appRef) -> Voyage (spec.slipwayRef).
 * An app can have several slipways and a slipway several voyages, so this
 * emits one pipeline per chain; an app or slipway with nothing downstream
 * still gets one entry.
 */
export function buildPipelines(
  apps: KoptanApp[],
  slipways: Slipway[],
  voyages: Voyage[],
): Pipeline[] {
  const group = <T>(items: T[], k: (item: T) => string) => {
    const m = new Map<string, T[]>();
    for (const i of items) m.set(k(i), [...(m.get(k(i)) ?? []), i]);
    return m;
  };
  const slipwaysByApp = group(slipways, (s) =>
    key(s.metadata.namespace, `${s.spec.appRef.kind}/${s.spec.appRef.name}`),
  );
  const voyagesBySlipway = group(voyages, (v) =>
    key(v.metadata.namespace, v.spec.slipwayRef.name),
  );
  return apps.flatMap((app) => {
    const mine =
      slipwaysByApp.get(
        key(app.metadata.namespace, `${app.kind}/${app.metadata.name}`),
      ) ?? [];
    if (mine.length === 0) return [{ app }];
    return mine.flatMap((slipway) => {
      const vs =
        voyagesBySlipway.get(
          key(slipway.metadata.namespace, slipway.metadata.name),
        ) ?? [];
      return vs.length === 0
        ? [{ app, slipway }]
        : vs.map((voyage) => ({ app, slipway, voyage }));
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
  apps: KoptanApp[],
  slipways: Slipway[],
  voyages: Voyage[],
): Overview {
  return {
    apps: { total: apps.length, byPhase: countByPhase(apps) },
    slipways: { total: slipways.length, byPhase: countByPhase(slipways) },
    voyages: { total: voyages.length, byPhase: countByPhase(voyages) },
    replicas: {
      desired: voyages.reduce((n, v) => n + (v.spec.replicas ?? 1), 0),
    },
  };
}

export async function loadAll(client: KoptanClient, namespace?: string) {
  const [appLists, slipways, voyages] = await Promise.all([
    Promise.all(APP_KINDS.map((k) => client.list(k, namespace))),
    client.list('Slipway', namespace),
    client.list('Voyage', namespace),
  ]);
  return {
    apps: appLists.flat().map(redactApp),
    slipways: slipways.map(redactSlipway),
    voyages: voyages.map(redactVoyage),
  };
}

const DNS_LABEL = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;

export function validateCreate(req: CreatePipelineRequest): string | undefined {
  if (!DNS_LABEL.test(req.name ?? '')) return 'name must be a DNS-1123 label';
  if (!APP_KINDS.includes(req.kind))
    return 'kind must be GoApp, JavaApp or DotnetApp';
  if (!req.source?.repo) return 'source.repo is required';
  if (!req.slipway?.registry || !req.slipway?.image)
    return 'slipway.registry and slipway.image are required';
  if (
    !Number.isInteger(req.voyage?.port) ||
    req.voyage.port < 1 ||
    req.voyage.port > 65535
  )
    return 'voyage.port must be 1-65535';
  if (!!req.slipway.username !== !!req.slipway.password)
    return 'slipway.username and slipway.password must be set together';
  if (req.voyage.healthCheckPath && !req.voyage.healthCheckPath.startsWith('/'))
    return 'voyage.healthCheckPath must start with /';
  if (
    req.voyage.replicas !== undefined &&
    (!Number.isInteger(req.voyage.replicas) || req.voyage.replicas < 0)
  )
    return 'voyage.replicas must be a non-negative integer';
  return undefined;
}

/** Creates the App, Slipway and Voyage, all named after the app. */
export async function createPipeline(
  client: KoptanClient,
  req: CreatePipelineRequest,
  createdBy: string,
): Promise<Pipeline> {
  const namespace = req.namespace ?? 'default';
  const metadata = {
    name: req.name,
    namespace,
    annotations: { 'koptan.felukka.org/created-by': createdBy },
  };
  // The operator reads the git token from a Secret named by source.patToken
  // (key "token"), so store it as one instead of putting it in the spec.
  const { patToken, ...source } = req.source;
  if (patToken) {
    await client.createSecret(namespace, `${req.name}-git`, {
      token: patToken,
    });
  }
  const app = await client.create(req.kind as AppKind, namespace, {
    metadata,
    spec: {
      ...req.appSpec,
      source: {
        ...source,
        ...(patToken ? { patToken: `${req.name}-git` } : {}),
      },
    },
  });
  const { registry, image, username, password } = req.slipway;
  const slipway = await client.create('Slipway', namespace, {
    metadata,
    spec: {
      appRef: { name: req.name, kind: req.kind },
      image: {
        registry,
        name: image,
        ...(username && password ? { creds: { username, password } } : {}),
      },
    },
  });
  const voyage = await client.create('Voyage', namespace, {
    metadata,
    spec: {
      slipwayRef: { name: req.name },
      port: req.voyage.port,
      replicas: req.voyage.replicas,
      ...(req.voyage.healthCheckPath
        ? { healthCheck: { path: req.voyage.healthCheckPath } }
        : {}),
    },
  });
  return {
    app: redactApp({ kind: req.kind, ...app }),
    slipway: redactSlipway(slipway),
    voyage: redactVoyage(voyage),
  };
}

type Stamped = {
  kind?: string;
  metadata: { name: string; namespace?: string; creationTimestamp?: string };
  status?: {
    phase?: string;
    error?: string;
    message?: string;
    latestImage?: string;
    deployedImage?: string;
    lastBuildTime?: string;
    conditions?: { lastTransitionTime?: string }[];
  };
};

/** Newest timestamp on a resource: last build, a condition, or creation. */
function lastTouched(r: Stamped): string {
  const times = [
    r.status?.lastBuildTime,
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
  if (kind === 'App') {
    return phase === 'Ready'
      ? { message: `Dockerfile ready for ${name}`, severity: 'success' }
      : { message: `${name} is ${phase ?? 'new'}`, severity: 'info' };
  }
  if (kind === 'Slipway') {
    return phase === 'Succeeded'
      ? {
          message: `Slipway build completed for ${name}${
            r.status?.latestImage ? ` (${r.status.latestImage})` : ''
          }`,
          severity: 'success',
        }
      : { message: `Slipway ${name} is ${phase ?? 'new'}`, severity: 'info' };
  }
  return phase === 'Running'
    ? {
        message: `Voyage ${name} is running${
          r.status?.deployedImage ? ` ${r.status.deployedImage}` : ''
        }`,
        severity: 'success',
      }
    : { message: `Voyage ${name} is ${phase ?? 'new'}`, severity: 'info' };
}

/** A recent-activity feed derived from the current state of each resource. */
export function buildActivity(
  apps: KoptanApp[],
  slipways: Slipway[],
  voyages: Voyage[],
  limit = 20,
): ActivityEntry[] {
  const entries = (
    [
      ['App', apps],
      ['Slipway', slipways],
      ['Voyage', voyages],
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
      `"${req.name}" already exists in namespace "${ns}" (an app, slipway, voyage or secret with that name)`,
    );
  }
  if (code === 403) {
    return new NotAllowedError(
      'The service account used by Backstage is not allowed to create these resources; see config/rbac/backstage_role.yaml',
    );
  }
  return e;
}
