import type {
  KoptanApp,
  Slipway,
  Voyage,
} from '@internal/plugin-koptan-common';
import {
  buildOverview,
  buildPipelines,
  redactApp,
  validateCreate,
} from './pipelines';

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
});
