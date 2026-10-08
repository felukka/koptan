import type {
  KoptanApp,
  Slipway,
  Voyage,
} from '@internal/plugin-koptan-common';
import {
  buildActivity,
  buildOverview,
  createPipeline,
  friendlyError,
  buildPipelines,
  redactApp,
  validateCreate,
} from './pipelines';
import type { KoptanClient, RawResource } from './crdClient';

const meta = (name: string) => ({ name, namespace: 'default' });
const app: KoptanApp = {
  kind: 'GoApp',
  metadata: meta('api'),
  spec: { source: { repo: 'https://x/y', patToken: 'secret' } },
  status: { phase: 'Ready' },
};
const slipway: Slipway = {
  metadata: meta('api-build'),
  spec: {
    appRef: { name: 'api', kind: 'GoApp' },
    image: { registry: 'r', name: 'n' },
  },
  status: { phase: 'Succeeded' },
};
const voyage: Voyage = {
  metadata: meta('api-run'),
  spec: { slipwayRef: { name: 'api-build' }, port: 8080, replicas: 3 },
  status: { phase: 'Running' },
};

describe('pipelines', () => {
  it('joins app, slipway and voyage', () => {
    const [p] = buildPipelines([app], [slipway], [voyage]);
    expect(p.slipway?.metadata.name).toBe('api-build');
    expect(p.voyage?.metadata.name).toBe('api-run');
  });

  it('emits one pipeline per slipway and voyage', () => {
    const slipway2: Slipway = { ...slipway, metadata: meta('api-build-2') };
    const voyage2: Voyage = { ...voyage, metadata: meta('api-run-2') };
    const out = buildPipelines([app], [slipway, slipway2], [voyage, voyage2]);
    expect(
      out.map((p) => [p.slipway?.metadata.name, p.voyage?.metadata.name]),
    ).toEqual([
      ['api-build', 'api-run'],
      ['api-build', 'api-run-2'],
      ['api-build-2', undefined],
    ]);
  });

  it('leaves unmatched links undefined', () => {
    const [p] = buildPipelines([app], [], [voyage]);
    expect(p.slipway).toBeUndefined();
    expect(p.voyage).toBeUndefined();
  });

  it('never exposes the PAT', () => {
    expect(JSON.stringify(redactApp(app))).not.toContain('secret');
  });

  it('drops the last-applied annotation and managedFields', () => {
    const raw = {
      ...app,
      metadata: {
        ...app.metadata,
        managedFields: [{}],
        annotations: {
          'kubectl.kubernetes.io/last-applied-configuration': 'secret',
          keep: 'me',
        },
      },
    };
    const out = redactApp(raw);
    expect(JSON.stringify(out)).not.toContain('secret');
    expect(out.metadata).toMatchObject({ annotations: { keep: 'me' } });
    expect(out.metadata).not.toHaveProperty('managedFields');
  });

  it('summarises phases and replicas', () => {
    const o = buildOverview([app], [slipway], [voyage]);
    expect(o.apps.byPhase).toEqual({ Ready: 1 });
    expect(o.replicas.desired).toBe(3);
  });

  it('validates create requests', () => {
    const ok = {
      name: 'api',
      kind: 'GoApp' as const,
      source: { repo: 'https://x/y' },
      slipway: { registry: 'r', image: 'n' },
      voyage: { port: 8080 },
    };
    expect(validateCreate(ok)).toBeUndefined();
    expect(validateCreate({ ...ok, name: 'Bad_Name' })).toMatch(/name/);
    expect(validateCreate({ ...ok, voyage: { port: 0 } })).toMatch(/port/);
  });

  it('rejects half-set registry credentials and a bad health path', () => {
    const ok = {
      name: 'api',
      kind: 'GoApp' as const,
      source: { repo: 'https://x/y' },
      slipway: { registry: 'r', image: 'n' },
      voyage: { port: 8080 },
    };
    expect(
      validateCreate({ ...ok, slipway: { ...ok.slipway, username: 'u' } }),
    ).toMatch(/together/);
    expect(
      validateCreate({
        ...ok,
        voyage: { port: 80, healthCheckPath: 'healthz' },
      }),
    ).toMatch(/healthCheckPath/);
  });

  it('builds a newest-first activity feed', () => {
    const feed = buildActivity(
      [
        {
          ...app,
          metadata: {
            ...app.metadata,
            creationTimestamp: '2026-01-01T00:00:00Z',
          },
        },
      ],
      [
        {
          ...slipway,
          status: {
            phase: 'Succeeded',
            latestImage: 'r/n:1',
            lastBuildTime: '2026-01-02T00:00:00Z',
          },
        },
      ],
      [{ ...voyage, status: { phase: 'Failed' } }],
    );
    expect(feed[0]).toMatchObject({ kind: 'Slipway', severity: 'success' });
    expect(feed[0].message).toContain('r/n:1');
    expect(feed.map((e) => e.kind)).toEqual(['Slipway', 'App', 'Voyage']);
    expect(feed.find((e) => e.kind === 'Voyage')?.severity).toBe('error');
  });

  it('stores the git token in a Secret and registry creds in the slipway', async () => {
    const created: [string, RawResource][] = [];
    const secrets: [string, Record<string, string>][] = [];
    const client: KoptanClient = {
      list: async () => [],
      create: async (kind, _ns, body) => {
        created.push([kind, body]);
        return body;
      },
      createSecret: async (_ns, name, data) => {
        secrets.push([name, data]);
      },
      clusterInfo: async () => ({ nodes: { total: 0, ready: 0 } }),
    };
    await createPipeline(
      client,
      {
        name: 'api',
        kind: 'GoApp',
        source: { repo: 'https://x/y', patToken: 'ghp_secret' },
        appSpec: { entrypoint: 'cmd/main.go' },
        slipway: { registry: 'r', image: 'n', username: 'u', password: 'p' },
        voyage: { port: 8080, healthCheckPath: '/healthz' },
      },
      'user:default/me',
    );
    expect(secrets).toEqual([['api-git', { token: 'ghp_secret' }]]);
    const [[, appBody], [, slipBody], [, voyBody]] = created;
    expect(JSON.stringify(appBody)).not.toContain('ghp_secret');
    expect(appBody.spec).toMatchObject({
      entrypoint: 'cmd/main.go',
      source: { patToken: 'api-git' },
    });
    expect(slipBody.spec.image.creds).toEqual({ username: 'u', password: 'p' });
    expect(voyBody.spec.healthCheck).toEqual({ path: '/healthz' });
  });

  it('explains 409 and 403 from the Kubernetes API', () => {
    const req = {
      name: 'api',
      kind: 'GoApp' as const,
      source: { repo: 'x' },
      slipway: { registry: 'r', image: 'n' },
      voyage: { port: 80 },
    };
    expect(friendlyError({ code: 409 }, req)).toMatchObject({
      name: 'ConflictError',
    });
    expect(String(friendlyError({ code: 409 }, req))).toContain(
      'already exists',
    );
    expect(friendlyError({ code: 403 }, req)).toMatchObject({
      name: 'NotAllowedError',
    });
    const other = new Error('boom');
    expect(friendlyError(other, req)).toBe(other);
  });
});
