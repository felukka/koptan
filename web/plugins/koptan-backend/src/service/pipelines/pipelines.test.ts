import type { CD, CI, Service } from '@internal/plugin-koptan-common';
import {
  buildActivity,
  buildOverview,
  buildPipelines,
  createPipeline,
  redactCI,
  redactService,
  validateCreate,
} from '.';
import { friendlyError } from '../k8s';
import { fakeClient } from '../testing/fakeClient';

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
    const ok = { name: 'api', repo: 'https://x.io/y' };
    expect(validateCreate(ok)).toBeUndefined();
    expect(
      validateCreate({ ...ok, repo: 'git@github.com:a/b.git' }),
    ).toBeUndefined();
    expect(
      validateCreate({ ...ok, repo: 'git://10.0.0.1:9418/a.git' }),
    ).toBeUndefined();
    expect(validateCreate({ ...ok, name: 'Bad_Name' })).toMatch(/name/);
    expect(validateCreate({ ...ok, namespace: 'Bad NS' })).toMatch(/namespace/);
    expect(validateCreate({ ...ok, repo: '' })).toMatch(/repo/);
    expect(validateCreate({ ...ok, env: [{ name: '' }] })).toMatch(/env/);
    expect(validateCreate({ ...ok, env: [{ name: '1A' }] })).toMatch(/env/);
    expect(validateCreate({ ...ok, replicas: -1 })).toMatch(/replicas/);
    expect(validateCreate({ ...ok, port: 0 })).toMatch(/port/);
  });

  it('rejects repo and revision strings that git could misread', () => {
    const ok = { name: 'api', repo: 'https://x.io/y' };
    for (const repo of [
      '--upload-pack=touch /tmp/pwned',
      'file:///etc',
      '/srv/repo.git',
      'ext::sh -c touch% /tmp/x',
      'https://x.io/a b',
      'ftp://x.io/y',
    ]) {
      expect(validateCreate({ ...ok, repo })).toMatch(/repo/);
    }
    for (const revision of ['--orphan', '-x', 'a..b', 'main;rm']) {
      expect(validateCreate({ ...ok, revision })).toMatch(/revision/);
    }
    expect(validateCreate({ ...ok, revision: 'feature/x' })).toBeUndefined();
  });

  it('checks registry fields', () => {
    const ok = { name: 'api', repo: 'https://x.io/y' };
    expect(
      validateCreate({
        ...ok,
        image: { registry: 'ghcr.io', repo: 'team/app' },
      }),
    ).toBeUndefined();
    expect(validateCreate({ ...ok, image: { username: 'u' } })).toMatch(
      /together/,
    );
    expect(
      validateCreate({ ...ok, image: { username: 'u', password: 'p' } }),
    ).toMatch(/registry/);
    expect(validateCreate({ ...ok, image: { repo: 'Team/App' } })).toMatch(
      /image.repo/,
    );
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

  it('stores secrets, references them and makes the Service own them', async () => {
    const { client, calls } = fakeClient();
    await createPipeline(
      client,
      {
        name: 'api',
        repo: 'https://x.io/y',
        revision: 'main',
        token: 'ghp_secret',
        env: [{ name: 'A', value: '1' }],
        image: {
          registry: 'ghcr.io',
          repo: 'team/api',
          username: 'u',
          password: 'p',
        },
        replicas: 2,
        port: 9090,
      },
      'user:default/me',
    );
    expect(calls.secrets.map(([n, , t]) => [n, t])).toEqual([
      ['api-git', undefined],
      ['api-registry', 'kubernetes.io/dockerconfigjson'],
    ]);
    const cfg = JSON.parse(calls.secrets[1][1]['.dockerconfigjson']);
    expect(cfg.auths['ghcr.io'].username).toBe('u');
    expect(calls.created).toHaveLength(1);
    const [[kind, body]] = calls.created;
    expect(kind).toBe('Service');
    expect(JSON.stringify(body)).not.toContain('ghp_secret');
    expect(JSON.stringify(body)).not.toContain('"p"');
    expect(body.spec).toMatchObject({
      source: {
        repo: 'https://x.io/y',
        revision: 'main',
        secretRef: { name: 'api-git', key: 'token' },
      },
      env: [{ name: 'A', value: '1' }],
      image: {
        registry: 'ghcr.io',
        repo: 'team/api',
        credentialsSecret: 'api-registry',
      },
      replicas: 2,
      port: 9090,
    });
    expect(calls.owners).toEqual([
      ['api-git', 'uid-1'],
      ['api-registry', 'uid-1'],
    ]);
  });

  it('deletes the secrets it created when the Service cannot be created', async () => {
    const { client, calls } = fakeClient({ failCreate: { code: 409 } });
    await expect(
      createPipeline(
        client,
        { name: 'api', repo: 'https://x.io/y', token: 't' },
        'user:default/me',
      ),
    ).rejects.toMatchObject({ code: 409 });
    expect(calls.deleted).toEqual(['api-git']);
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
    expect(
      String(
        friendlyError(
          {
            code: 404,
            body: '{"message":"namespaces \\"team-z\\" not found"}',
          },
          { ...req, namespace: 'team-z' },
        ),
      ),
    ).toContain('Namespace "team-z" does not exist');
    expect(
      friendlyError(
        { code: 422, body: { message: 'spec.port: Invalid' } },
        req,
      ),
    ).toMatchObject({ name: 'InputError', message: 'spec.port: Invalid' });
    const other = new Error('boom');
    expect(friendlyError(other, req)).toBe(other);
  });
});
