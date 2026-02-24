import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { test } from 'node:test';
import { runAgent } from '../src/agent/loop.js';
import { parsePlan } from '../src/agent/plan.js';
import { createTools } from '../src/agent/tools/index.js';
import { ToolsUnsupportedError } from '../src/providers/errors.js';
import { fakeProvider } from '../src/providers/fake.js';
import { tempDir } from './helpers.js';

const call = (name, input, id = name) => ({ toolCalls: [{ id, name, input }] });

async function setup(replies, opts) {
  const root = await tempDir();
  const provider = fakeProvider(replies, opts);
  const events = [];
  const base = {
    provider,
    root,
    system: 'sys',
    prompt: 'make a server',
    maxSteps: 5,
    tools: createTools({ root, allowCommands: false }),
    onEvent: (e) => events.push(e),
  };
  return { root, provider, events, base };
}

test('runs tools until the model answers', async () => {
  const { root, provider, events, base } = await setup([
    call('write_file', {
      path: 'server.js',
      content: 'listen(process.env.PORT)',
    }),
    call('read_file', { path: 'missing.js' }),
    { text: 'Added server.js' },
  ]);
  const { summary } = await runAgent(base);
  assert.equal(summary, 'Added server.js');
  assert.equal(await fs.readFile(path.join(root, 'server.js'), 'utf8'), 'listen(process.env.PORT)');
  // The failed read went back to the model as an error result.
  const third = provider.requests[2].messages.at(-1);
  assert.equal(third.role, 'tool');
  assert.equal(third.results[0].isError, true);
  assert.deepEqual(
    events.filter((e) => e.type === 'tool').map((e) => e.name),
    ['write_file', 'read_file'],
  );
});

test('stops on refusals, truncation and the step limit', async () => {
  let { base } = await setup([{ text: 'no', stop: 'refusal' }]);
  await assert.rejects(runAgent(base), /declined/);
  ({ base } = await setup([{ ...call('write_file', { path: 'a', content: 'x' }), stop: 'max_tokens' }]));
  await assert.rejects(runAgent(base), /cut off/);
  ({ base } = await setup(Array.from({ length: 6 }, (_, i) => call('list_files', {}, `c${i}`))));
  await assert.rejects(runAgent(base), /after 5 steps/);
});

test('falls back to an edit plan without tool calling', async () => {
  const plan = JSON.stringify({
    summary: 'Added app',
    changes: [{ path: 'app.py', action: 'write', content: 'print(1)' }],
  });
  const { root, provider, base } = await setup([
    () => {
      throw new ToolsUnsupportedError('no tools');
    },
    { text: `\`\`\`json\n${plan}\n\`\`\`` },
  ]);
  const { summary } = await runAgent(base);
  assert.equal(summary, 'Added app');
  assert.equal(await fs.readFile(path.join(root, 'app.py'), 'utf8'), 'print(1)');
  assert.ok(provider.requests[1].schema, 'plan mode asks for structured output');
  assert.throws(() => parsePlan('{"summary": 1}'), /summary and changes/);
  assert.throws(() => parsePlan('{"summary":"s","changes":[{"path":"a","action":"chmod"}]}'), /action/);
});
