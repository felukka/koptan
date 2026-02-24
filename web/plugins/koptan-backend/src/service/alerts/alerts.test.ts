import type { CreateAlertRequest } from '@internal/plugin-koptan-common';
import { fakeClient } from '../testing/fakeClient';
import {
  buildDeliveries,
  createAlert,
  redactAlert,
  validateCreateAlert,
} from '.';

const request: CreateAlertRequest = {
  name: 'payments',
  service: 'api',
  events: ['CIFailed', 'CDFailed'],
  channels: [
    { name: 'slack', type: 'slack', url: 'https://hooks.slack.com/T/B/SECRET' },
    {
      name: 'hook',
      type: 'webhook',
      url: 'http://receiver.tools.svc/koptan',
      signingKey: 'k3y',
    },
  ],
};

describe('alerts', () => {
  it('stores channel URLs in an owned Secret and only references them', async () => {
    const { client, calls } = fakeClient();
    const alert = await createAlert(client, request, 'user:default/me');
    const [[name, data]] = calls.secrets;
    expect(name).toBe('payments-channels');
    expect(data).toEqual({
      'slack-url': 'https://hooks.slack.com/T/B/SECRET',
      'hook-url': 'http://receiver.tools.svc/koptan',
      'hook-signing': 'k3y',
    });
    const [[kind, body]] = calls.created;
    expect(kind).toBe('Alert');
    expect(JSON.stringify(body)).not.toContain('SECRET');
    expect(JSON.stringify(body)).not.toContain('k3y');
    expect(body.spec.channels[1]).toEqual({
      name: 'hook',
      type: 'webhook',
      urlSecretRef: { name: 'payments-channels', key: 'hook-url' },
      signingSecretRef: { name: 'payments-channels', key: 'hook-signing' },
    });
    expect(calls.owners).toEqual([['payments-channels', 'uid-1']]);
    expect(alert.metadata.name).toBe('payments');
  });

  it('validates the request like the operator', () => {
    expect(validateCreateAlert(request)).toBeUndefined();
    const bad = (patch: Partial<CreateAlertRequest>) =>
      validateCreateAlert({ ...request, ...patch });
    expect(bad({ service: undefined })).toContain('service or a label');
    expect(bad({ events: ['Nope' as never] })).toContain('unknown event');
    expect(
      bad({ channels: [{ name: 's', type: 'slack', url: 'http://x.io' }] }),
    ).toContain('https');
    expect(
      bad({
        channels: [
          { name: 's', type: 'slack', url: 'https://x.io', signingKey: 'k' },
        ],
      }),
    ).toContain('only webhook');
    expect(bad({ channels: [] })).toContain('one to ten');
    expect(
      bad({ selector: { 'bad key!': 'x' }, service: undefined }),
    ).toContain('not a valid label');
  });

  it('hides dedupe state and merges deliveries newest first', () => {
    const a = redactAlert({
      metadata: { name: 'a' },
      spec: { channels: [] },
      status: {
        lastNotified: { 'api/Push/slack': 'x' },
        deliveries: [
          { time: '2026-01-01T00:00:02Z', event: 'Push', service: 'api' },
        ],
      },
    });
    const b = redactAlert({
      metadata: { name: 'b' },
      spec: { channels: [] },
      status: {
        deliveries: [
          { time: '2026-01-01T00:00:05Z', event: 'CIFailed', service: 'web' },
        ],
      },
    });
    expect(JSON.stringify(a)).not.toContain('lastNotified');
    expect(buildDeliveries([a, b]).map((d) => d.alert)).toEqual(['b', 'a']);
  });
});
