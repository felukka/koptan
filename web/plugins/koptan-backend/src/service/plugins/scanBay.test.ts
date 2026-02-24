import { fakeClient } from '../testing/fakeClient';
import { loadScanBay, redactPlugin } from '.';

describe('scan bay', () => {
  it('never returns custom step env', () => {
    const p = redactPlugin({
      metadata: { name: 'trivy' },
      spec: {
        type: 'custom',
        custom: {
          image: 'trivy',
          env: [{ name: 'TOKEN', value: 'secret-value' }],
          envFrom: [{ secretRef: { name: 'x' } }],
        },
      },
    });
    expect(JSON.stringify(p)).not.toContain('secret-value');
    expect(p.spec.custom).toEqual({ image: 'trivy' });
  });

  it('sorts plugins by order and lists the runs of CIs with plugin results', async () => {
    const { client } = fakeClient({
      objects: {
        CIPlugin: [
          { metadata: { name: 'b' }, spec: { type: 'custom', order: 50 } },
          { metadata: { name: 'a' }, spec: { type: 'codeql', order: 10 } },
        ],
        CI: [
          {
            metadata: { name: 'api-ci', namespace: 'default' },
            spec: { service: { name: 'api' }, image: {}, revision: 'abc' },
            status: {
              phase: 'Failed',
              pluginResults: [{ name: 'a', phase: 'Failed', message: 'x' }],
            },
          },
          {
            metadata: { name: 'web-ci' },
            spec: { service: { name: 'web' }, image: {} },
            status: { phase: 'Succeeded' },
          },
        ],
      },
    });
    const bay = await loadScanBay(client);
    expect(bay.plugins.map((p) => p.metadata.name)).toEqual(['a', 'b']);
    expect(bay.runs).toEqual([
      {
        service: 'api',
        namespace: 'default',
        ci: 'api-ci',
        revision: 'abc',
        phase: 'Failed',
        results: [{ name: 'a', phase: 'Failed', message: 'x' }],
      },
    ]);
  });
});
