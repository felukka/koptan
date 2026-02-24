// The HTTP API the Backstage backend calls through the Kubernetes service
// proxy. POST /runs streams progress as server-sent events.

import { timingSafeEqual } from 'node:crypto';
import http from 'node:http';
import { log } from './log.js';
import { BusyError, MAX_PROMPT } from './session.js';

const MAX_BODY = 64 * 1024;

const json = (res, status, body) => {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(body));
};

/** The token travels in its own header: the Kubernetes API server's
 * service proxy drops Authorization, which it reads as its own credentials. */
export const TOKEN_HEADER = 'x-koptan-agent-token';

function authorized(req, token) {
  const given = Buffer.from(String(req.headers[TOKEN_HEADER] ?? ''));
  const want = Buffer.from(token);
  return given.length === want.length && timingSafeEqual(given, want);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const chunks = [];
    req.on('data', (c) => {
      size += c.length;
      if (size > MAX_BODY) {
        reject(new Error('request body is too large'));
        req.destroy();
      } else chunks.push(c);
    });
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

async function startRun(session, req, res) {
  let prompt;
  try {
    prompt = JSON.parse(await readBody(req)).prompt;
  } catch {
    return json(res, 400, { error: 'send JSON {"prompt": "..."}' });
  }
  if (typeof prompt !== 'string' || !prompt.trim() || prompt.length > MAX_PROMPT) {
    return json(res, 400, {
      error: `prompt must be 1-${MAX_PROMPT} characters`,
    });
  }
  if (session.busy) return json(res, 409, { error: 'a run is already in progress' });
  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive',
  });
  const send = (e) => {
    if (!res.writableEnded) res.write(`event: ${e.type}\ndata: ${JSON.stringify(e)}\n\n`);
  };
  try {
    await session.run(prompt.trim(), send);
  } catch (e) {
    if (!(e instanceof BusyError)) throw e;
    send({ type: 'error', message: e.message });
  }
  res.end();
}

/** Creates (but does not start) the server. */
export function createServer(session, token) {
  return http.createServer(async (req, res) => {
    const { pathname } = new URL(req.url ?? '/', 'http://agent');
    try {
      if (req.method === 'GET' && pathname === '/healthz') return json(res, 200, { ok: true, busy: session.busy });
      if (!authorized(req, token)) return json(res, 401, { error: 'unauthorized' });
      if (req.method === 'GET' && pathname === '/runs') return json(res, 200, session.runs);
      if (req.method === 'POST' && pathname === '/runs') return await startRun(session, req, res);
      return json(res, 404, { error: 'not found' });
    } catch (e) {
      log('error', 'request failed', { error: e.message });
      if (!res.headersSent) json(res, 500, { error: 'internal error' });
      else res.end();
    }
  });
}
