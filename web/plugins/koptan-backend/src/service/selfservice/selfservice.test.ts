import { Readable } from 'node:stream';
import type { CreateSelfServiceRequest } from '@internal/plugin-koptan-common';
import { fakeClient } from '../testing/fakeClient';
import {
  AgentClient,
  agentToken,
  createSelfService,
  validateCreateSelfService,
  watchDone,
} from '.';

const request: CreateSelfServiceRequest = {
  name: 'shop',
  repo: {
    mode: 'create',
    provider: 'github',
    owner: 'team',
    token: 'ghp_SECRET',
  },
  ai: {
    provider: 'anthropic',
    model: 'claude-opus-5-5',
    apiKey: 'sk-ant-SECRET',
  },
  port: 3000,
};

describe('self service', () => {
  it('derives the same agent token as the operator', () => {
    // Pinned in internal/controller/agent_token_test.go as well.
    expect(agentToken('koptan-test-key', 'team', 'shop')).toBe(
      '4db4b6dffe33b18525ca87a81968e519a1749b3658c5f44ee1cb3c2e63c954e9',
    );
  });

  it('keeps tokens in owned Secrets and only references them', async () => {
    const { client, calls } = fakeClient();
    await createSelfService(client, request, 'user:default/me');
    expect(
      calls.secrets.map(([name, data]) => [name, Object.keys(data)]),
    ).toEqual([
      ['shop-git', ['token']],
      ['shop-ai', ['apiKey']],
    ]);
    const [[kind, body]] = calls.created;
    expect(kind).toBe('SelfService');
    expect(JSON.stringify(body)).not.toContain('SECRET');
    expect(body.spec.repo.create).toEqual({
      provider: 'github',
      owner: 'team',
      private: true,
      tokenSecretRef: { name: 'shop-git', key: 'token' },
    });
    expect(body.spec.ai.apiKeySecretRef).toEqual({
      name: 'shop-ai',
      key: 'apiKey',
    });
    expect(calls.owners.map(([n]) => n)).toEqual(['shop-git', 'shop-ai']);
  });

  it('validates repositories, models and branches', () => {
    expect(validateCreateSelfService(request)).toBeUndefined();
    const bad = (patch: Partial<CreateSelfServiceRequest>) =>
      validateCreateSelfService({ ...request, ...patch });
    expect(
      bad({ repo: { mode: 'existing', url: 'http://x.io/r.git', token: 't' } }),
    ).toContain('https');
    expect(
      bad({
        repo: { mode: 'existing', url: 'https://u:p@x.io/r.git', token: 't' },
      }),
    ).toContain('without credentials');
    expect(
      bad({ repo: { mode: 'create', provider: 'github', token: '' } }),
    ).toContain('token');
    expect(
      bad({ ai: { provider: 'openai-compatible', model: 'llama' } }),
    ).toContain('baseURL');
    expect(bad({ branch: 'a..b' })).toContain('branch');
  });

  it('calls the agent through the service proxy with its token', async () => {
    const { client, calls } = fakeClient({
      proxyReply: { status: 200, body: '[{"id":"r1","status":"succeeded"}]' },
    });
    const runs = await new AgentClient(client, 'koptan-test-key').runs(
      'team',
      'shop',
    );
    expect(runs[0].id).toBe('r1');
    const [[target, init]] = calls.proxied;
    expect(target).toEqual({
      namespace: 'team',
      service: 'shop-agent',
      port: 8080,
      path: '/runs',
    });
    expect(init.headers?.['X-Koptan-Agent-Token']).toBe(
      agentToken('koptan-test-key', 'team', 'shop'),
    );
    expect(init.headers?.Authorization).toBeUndefined();
    await expect(
      new AgentClient(client, undefined).runs('team', 'shop'),
    ).rejects.toThrow('koptan.agentKey');
  });

  it('passes events through and reports the pushed commit', async () => {
    const commits: string[] = [];
    const frames = [
      'event: status\ndata: {"type":"status","text":"x"}\n\n',
      'event: done\ndata: {"type":"done","com',
      'mit":"abc123","summary":"s"}\n\n',
    ];
    let out = '';
    for await (const chunk of Readable.from(frames).pipe(
      watchDone((c) => commits.push(c)),
    )) {
      out += chunk.toString();
    }
    expect(out).toBe(frames.join(''));
    expect(commits).toEqual(['abc123']);
  });
});
