import { createHmac } from 'node:crypto';
import { Transform } from 'node:stream';
import type { AgentRun } from '@internal/plugin-koptan-common';
import type { KoptanClient, ProxyResponse } from '../k8s';

const AGENT_PORT = 8080;
/** Changing it makes the operator reconcile, and so poll git, at once. */
export const REFRESH_ANNOTATION = 'koptan.felukka.org/refresh';

/**
 * The agent's bearer token: hex(HMAC-SHA256(key, "<namespace>/<name>")),
 * mirroring AgentToken in the operator, so no Secret has to be read.
 */
export function agentToken(key: string, namespace: string, name: string) {
  return createHmac('sha256', key).update(`${namespace}/${name}`).digest('hex');
}

export class AgentClient {
  constructor(
    private readonly client: KoptanClient,
    private readonly key: string | undefined,
  ) {}

  private target(namespace: string, name: string, path: string) {
    return { namespace, service: `${name}-agent`, port: AGENT_PORT, path };
  }

  private headers(namespace: string, name: string) {
    if (!this.key) {
      throw new Error(
        'koptan.agentKey is not configured; it must match the operator KOPTAN_AGENT_KEY',
      );
    }
    return {
      Authorization: `Bearer ${agentToken(this.key, namespace, name)}`,
      'Content-Type': 'application/json',
    };
  }

  async runs(namespace: string, name: string): Promise<AgentRun[]> {
    const res = await this.client.proxy(this.target(namespace, name, '/runs'), {
      method: 'GET',
      headers: this.headers(namespace, name),
    });
    const text = await readAll(res);
    if (res.status !== 200) throw agentError(res.status, text);
    return JSON.parse(text) as AgentRun[];
  }

  /** Starts a run; the response body is the agent's event stream. */
  async start(
    namespace: string,
    name: string,
    prompt: string,
    signal: AbortSignal,
  ): Promise<ProxyResponse> {
    const res = await this.client.proxy(this.target(namespace, name, '/runs'), {
      method: 'POST',
      headers: this.headers(namespace, name),
      body: JSON.stringify({ prompt }),
      signal,
    });
    if (res.status !== 200) throw agentError(res.status, await readAll(res));
    return res;
  }
}

/**
 * Passes server-sent events through unchanged and calls onDone with the
 * commit of a `done` event.
 */
export function watchDone(onDone: (commit: string) => void): Transform {
  let buffer = '';
  return new Transform({
    transform(chunk, _enc, next) {
      buffer += chunk.toString('utf8');
      let end = buffer.indexOf('\n\n');
      while (end >= 0) {
        const frame = buffer.slice(0, end);
        buffer = buffer.slice(end + 2);
        if (frame.startsWith('event: done\n')) {
          try {
            const data = JSON.parse(frame.slice(frame.indexOf('data: ') + 6));
            if (data.commit) onDone(data.commit);
          } catch {
            // A malformed frame is still passed on; the UI shows it.
          }
        }
        end = buffer.indexOf('\n\n');
      }
      next(null, chunk);
    },
  });
}

async function readAll(res: ProxyResponse): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const c of res.body) chunks.push(Buffer.from(c));
  return Buffer.concat(chunks).toString('utf8');
}

/** An error carrying the agent's status, for the route to translate. */
function agentError(status: number, text: string) {
  let message = text.slice(0, 300);
  try {
    message = JSON.parse(text).error ?? message;
  } catch {
    // Not JSON: keep the text.
  }
  return Object.assign(new Error(`agent answered ${status}: ${message}`), {
    agentStatus: status,
  });
}
