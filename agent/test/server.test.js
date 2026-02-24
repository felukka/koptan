import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createServer } from '../src/server.js';

const TOKEN = 'a-very-long-agent-token';

function fakeSession() {
  return {
    busy: false,
    runs: [{ id: 'r1', status: 'succeeded' }],
    async run(prompt, onEvent) {
      this.busy = true;
      onEvent({ type: 'status', text: 'working' });
      await new Promise((r) => setTimeout(r, 30));
      onEvent({ type: 'done', commit: 'abc', summary: `did ${prompt}` });
      this.busy = false;
    },
  };
}

async function start() {
  const server = createServer(fakeSession(), TOKEN);
  await new Promise((r) => server.listen(0, r));
  const base = `http://127.0.0.1:${server.address().port}`;
  const call = (path, init = {}, token = TOKEN) =>
    fetch(base + path, {
      ...init,
      headers: {
        ...(token ? { 'X-Koptan-Agent-Token': token } : {}),
        ...init.headers,
      },
    });
  return { server, call };
}

test('health is open, everything else needs the token', async (t) => {
  const { server, call } = await start();
  t.after(() => server.close());
  assert.equal((await call('/healthz', {}, '')).status, 200);
  assert.equal((await call('/runs', {}, '')).status, 401);
  assert.equal((await call('/runs', {}, 'wrong-token-of-same-size!')).status, 401);
  assert.deepEqual(await (await call('/runs')).json(), [{ id: 'r1', status: 'succeeded' }]);
});

test('POST /runs streams events, validates the prompt and refuses a second run', async (t) => {
  const { server, call } = await start();
  t.after(() => server.close());
  const post = (body) =>
    call('/runs', {
      method: 'POST',
      body,
      headers: { 'Content-Type': 'application/json' },
    });
  assert.equal((await post('{"prompt": ""}')).status, 400);
  assert.equal((await post('not json')).status, 400);
  assert.equal((await post(JSON.stringify({ prompt: 'x'.repeat(8001) }))).status, 400);

  const res = await post(JSON.stringify({ prompt: 'add a page' }));
  assert.equal(res.headers.get('content-type'), 'text/event-stream');
  const busy = await post(JSON.stringify({ prompt: 'another' }));
  assert.equal(busy.status, 409);
  const text = await res.text();
  assert.match(text, /event: status\ndata: \{"type":"status","text":"working"\}/);
  assert.match(text, /event: done\ndata: .*"summary":"did add a page"/);
});
