import {
  type ActivityEntry,
  type CD,
  type CI,
  type CreatePipelineRequest,
  KOPTAN_GROUP,
  KOPTAN_VERSION,
  type Overview,
  type Pipeline,
  type Service,
} from '@internal/plugin-koptan-common';
import { ConflictError, InputError, NotAllowedError } from '@backstage/errors';
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
const SCP_URL = /^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._/~-]+$/;
const REVISION = /^[A-Za-z0-9._/][A-Za-z0-9._/-]*$/;
const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
const REGISTRY = /^[A-Za-z0-9.-]+(:[0-9]+)?(\/[A-Za-z0-9._-]+)*$/;
const IMAGE_REPO =
  /^[a-z0-9]+([._-][a-z0-9]+)*(\/[a-z0-9]+([._-][a-z0-9]+)*)*$/;

/**
 * Only remote git URLs: https, http, ssh, git, or user@host:path. Rejects
 * local paths, file://, ext:: and anything git could read as an option.
 * Mirrors ValidateGitURL in the operator (internal/utils/url.go).
 */
export function isValidRepoUrl(repo: string): boolean {
  if (!repo || repo.startsWith('-') || /\s/.test(repo)) return false;
  if (SCP_URL.test(repo)) return true;
  try {
    const u = new URL(repo);
    return (
      ['https:', 'http:', 'ssh:', 'git:'].includes(u.protocol) &&
      !!u.hostname &&
      u.pathname.length > 1
    );
  } catch {
    return false;
  }
}

export function validateCreate(req: CreatePipelineRequest): string | undefined {
  if (!DNS_LABEL.test(req.name ?? '')) return 'name must be a DNS-1123 label';
  if (req.namespace !== undefined && !DNS_LABEL.test(req.namespace))
    return 'namespace must be a DNS-1123 label';
  if (!req.repo) return 'repo is required';
  if (!isValidRepoUrl(req.repo))
    return 'repo must be an https://, http://, ssh:// or git:// URL, or user@host:path';
  if (
    req.revision &&
    (req.revision.length > 250 ||
      !REVISION.test(req.revision) ||
      req.revision.includes('..'))
  )
    return 'revision must be a branch, tag or commit SHA';
  for (const e of req.env ?? []) {
    if (!ENV_NAME.test(e.name ?? ''))
      return `env variable name "${e.name ?? ''}" is not valid`;
  }
  const img = req.image;
  if (img) {
    if (img.registry && !REGISTRY.test(img.registry))
      return 'image.registry must be a registry host, e.g. ghcr.io';
    if (img.repo && !IMAGE_REPO.test(img.repo))
      return 'image.repo must be a lowercase image path, e.g. team/app';
    if (!!img.username !== !!img.password)
      return 'registry username and password must be set together';
    if (img.username && !img.registry)
      return 'set image.registry when giving registry credentials';
  }
  if (
    req.replicas !== undefined &&
    (!Number.isInteger(req.replicas) || req.replicas < 0 || req.replicas > 50)
  )
    return 'replicas must be a whole number from 0 to 50';
  if (
    req.port !== undefined &&
    (!Number.isInteger(req.port) || req.port < 1 || req.port > 65535)
  )
    return 'port must be 1-65535';
  return undefined;
}

/** A .dockerconfigjson for one registry. */
export function dockerConfigJson(
  registry: string,
  username: string,
  password: string,
): string {
  const auth = Buffer.from(`${username}:${password}`).toString('base64');
  return JSON.stringify({
    auths: { [registry]: { username, password, auth } },
  });
}

/**
 * Creates the Service; the operator derives its CI and CD. Secrets come
 * first so the operator can read them, are deleted again if the Service
 * cannot be created, and are owned by the Service once it exists.
 */
export async function createPipeline(
  client: KoptanClient,
  req: CreatePipelineRequest,
  createdBy: string,
): Promise<Pipeline> {
  const namespace = req.namespace ?? 'default';
  const created: string[] = [];
  const rollback = async () => {
    for (const name of created) {
      await client.deleteSecret(namespace, name).catch(() => undefined);
    }
  };
  const gitSecret = `${req.name}-git`;
  const registrySecret = `${req.name}-registry`;
  const img = req.image;

  let service: RawResource;
  try {
    if (req.token) {
      await client.createSecret(namespace, gitSecret, { token: req.token });
      created.push(gitSecret);
    }
    if (img?.username && img.password && img.registry) {
      await client.createSecret(
        namespace,
        registrySecret,
        {
          '.dockerconfigjson': dockerConfigJson(
            img.registry,
            img.username,
            img.password,
          ),
        },
        'kubernetes.io/dockerconfigjson',
      );
      created.push(registrySecret);
    }
    const image = {
      ...(img?.registry ? { registry: img.registry } : {}),
      ...(img?.repo ? { repo: img.repo } : {}),
      ...(created.includes(registrySecret)
        ? { credentialsSecret: registrySecret }
        : {}),
    };
    service = await client.create('Service', namespace, {
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
            ? { secretRef: { name: gitSecret, key: 'token' } }
            : {}),
        },
        ...(req.env?.length ? { env: req.env } : {}),
        ...(Object.keys(image).length ? { image } : {}),
        ...(req.replicas !== undefined ? { replicas: req.replicas } : {}),
        ...(req.port !== undefined ? { port: req.port } : {}),
      },
    });
  } catch (e) {
    await rollback();
    throw e;
  }

  const uid = service.metadata?.uid;
  if (uid) {
    const owner = {
      apiVersion: `${KOPTAN_GROUP}/${KOPTAN_VERSION}`,
      kind: 'Service',
      name: req.name,
      uid,
    };
    await Promise.all(
      created.map((name) => client.setSecretOwner(namespace, name, owner)),
    );
  }
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
  const err = e as { code?: number; body?: unknown };
  const ns = req.namespace ?? 'default';
  const apiMessage = () => {
    try {
      const body =
        typeof err.body === 'string' ? JSON.parse(err.body) : err.body;
      return (body as { message?: string })?.message;
    } catch {
      return undefined;
    }
  };
  if (err.code === 409) {
    return new ConflictError(
      `"${req.name}" already exists in namespace "${ns}" (a service or secret with that name)`,
    );
  }
  if (err.code === 403) {
    return new NotAllowedError(
      'The service account used by Backstage is not allowed to create these resources; see config/rbac/backstage_role.yaml',
    );
  }
  if (err.code === 404) {
    const msg = apiMessage() ?? '';
    return new InputError(
      /namespaces? .* not found/.test(msg)
        ? `Namespace "${ns}" does not exist`
        : `Not found: ${msg || 'the Koptan CRDs may not be installed in this cluster'}`,
    );
  }
  if (err.code === 422 || err.code === 400) {
    return new InputError(apiMessage() ?? 'The cluster rejected the request');
  }
  return e;
}
