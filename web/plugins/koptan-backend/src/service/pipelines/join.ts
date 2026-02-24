import type {
  CD,
  CI,
  Overview,
  Pipeline,
  Service,
} from '@internal/plugin-koptan-common';
import type { KoptanClient } from '../k8s';
import { redactCD, redactCI, redactService } from './redact';

const key = (ns: string | undefined, name: string) => `${ns ?? ''}/${name}`;

const group = <T>(items: T[], k: (item: T) => string) => {
  const m = new Map<string, T[]>();
  for (const i of items) m.set(k(i), [...(m.get(k(i)) ?? []), i]);
  return m;
};

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

export function countByPhase(items: { status?: { phase?: string } }[]) {
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
