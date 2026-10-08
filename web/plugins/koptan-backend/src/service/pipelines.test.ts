import type { CD, CI, Service } from '@internal/plugin-koptan-common';
import {
  buildActivity,
  buildOverview,
  buildPipelines,
  createPipeline,
  friendlyError,
  redactCI,
  redactService,
  validateCreate,
} from './pipelines';
import type { KoptanClient, RawResource } from './crdClient';

const meta = (name: string) => ({ name, namespace: 'default' });
const service: Service = {
  metadata: meta('api'),
  spec: {
    source: {
      repo: 'https://x/y',
      secretRef: { name: 'api-git', key: 'token' },
    },
  },
  status: { phase: 'Ready', serviceType: 'go' },
};
const ci: CI = {
  metadata: meta('api-ci'),
  spec: { service: { name: 'api' }, image: { registry: 'r', repo: 'n' } },
  status: { phase: 'Succeeded' },
};
const cd: CD = {
  metadata: meta('api-cd'),
  spec: { ci: { name: 'api-ci' }, replicas: 3 },
  status: { phase: 'Running' },
};

describe('pipelines', () => {
  it('joins service, ci and cd', () => {
    const [p] = buildPipelines([service], [ci], [cd]);
    expect(p.ci?.metadata.name).toBe('api-ci');
    expect(p.cd?.metadata.name).toBe('api-cd');
  });

  it('emits one pipeline per ci and cd', () => {
    const ci2: CI = { ...ci, metadata: meta('api-ci-2') };
    const cd2: CD = { ...cd, metadata: meta('api-cd-2') };
    const out = buildPipelines([service], [ci, ci2], [cd, cd2]);
    expect(out.map((p) => [p.ci?.metadata.name, p.cd?.metadata.name])).toEqual([
      ['api-ci', 'api-cd'],
      ['api-ci', 'api-cd-2'],
      ['api-ci-2', undefined],
    ]);
  });

  it('leaves unmatched links undefined', () => {
    const [p] = buildPipelines([service], [], [cd]);
    expect(p.ci).toBeUndefined();
    expect(p.cd).toBeUndefined();
  });

  it('never exposes registry login', () => {
    const raw = {
      ...ci,
      spec: {
        ...ci.spec,
        image: { ...ci.spec.image, loginSecret: { password: 'c2VjcmV0' } },
      },
    };
    expect(JSON.stringify(redactCI(raw))).not.toContain('c2VjcmV0');
  });

  it('drops the last-applied annotation and managedFields', () => {
    const raw = {
      ...service,
      metadata: {
        ...service.metadata,
        managedFields: [{}],
        annotations: {
          'kubectl.kubernetes.io/last-applied-configuration': 'LEAKED',
          keep: 'me',
        },
      },
    };
    const out = redactService(raw);
    expect(JSON.stringify(out)).not.toContain('LEAKED');
    expect(out.metadata).toMatchObject({ annotations: { keep: 'me' } });
    expect(out.metadata).not.toHaveProperty('managedFields');
  });

  it('summarises phases and replicas', () => {
    const o = buildOverview([service], [ci], [cd]);
    expect(o.services.byPhase).toEqual({ Ready: 1 });
    expect(o.replicas.desired).toBe(3);
  });

  it('validates create requests', () => {
    const ok = { name: 'api', repo: 'https://x/y' };
    expect(validateCreate(ok)).toBeUndefined();
    expect(validateCreate({ ...ok, name: 'Bad_Name' })).toMatch(/name/);
    expect(validateCreate({ ...ok, repo: '' })).toMatch(/repo/);
    expect(validateCreate({ ...ok, env: [{ name: '' }] })).toMatch(/env/);
  });

  it('builds a newest-first activity feed', () => {
    const feed = buildActivity(
      [
        {
          ...service,
          metadata: {
            ...service.metadata,
            creationTimestamp: '2026-01-01T00:00:00Z',
          },
        },
      ],
      [
        {
          ...ci,
          status: {
            phase: 'Succeeded',
            latestImage: 'r/n:1',
            lastBuildTime: '2026-01-02T00:00:00Z',
          },
        },
      ],
      [{ ...cd, status: { phase: 'Failed' } }],
    );
    expect(feed[0]).toMatchObject({ kind: 'CI', severity: 'success' });
    expect(feed[0].message).toContain('r/n:1');
    expect(feed.map((e) => e.kind)).toEqual(['CI', 'Service', 'CD']);
    expect(feed.find((e) => e.kind === 'CD')?.severity).toBe('error');
  });

  it('stores the git token in a Secret and references it', async () => {
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
        repo: 'https://x/y',
        revision: 'main',
        token: 'ghp_secret',
        env: [{ name: 'A', value: '1' }],
      },
      'user:default/me',
    );
    expect(secrets).toEqual([['api-git', { token: 'ghp_secret' }]]);
    expect(created).toHaveLength(1);
    const [[kind, body]] = created;
    expect(kind).toBe('Service');
    expect(JSON.stringify(body)).not.toContain('ghp_secret');
    expect(body.spec).toMatchObject({
      source: {
        repo: 'https://x/y',
        revision: 'main',
        secretRef: { name: 'api-git', key: 'token' },
      },
      env: [{ name: 'A', value: '1' }],
    });
  });

  it('explains 409 and 403 from the Kubernetes API', () => {
    const req = { name: 'api', repo: 'x' };
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
