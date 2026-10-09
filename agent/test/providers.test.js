import assert from 'node:assert/strict';
import { test } from 'node:test';
import { anthropicProvider, toAnthropicMessages } from '../src/providers/anthropic.js';
import { ToolsUnsupportedError } from '../src/providers/errors.js';
import { openAIProvider, toOpenAIMessages } from '../src/providers/openai.js';

const history = [
  { role: 'user', text: 'hi' },
  {
    role: 'assistant',
    text: 'ok',
    toolCalls: [{ id: 't1', name: 'read_file', input: { path: 'a' } }],
  },
  {
    role: 'tool',
    results: [{ id: 't1', name: 'read_file', output: 'nope', isError: true }],
  },
];
const tools = [
  {
    name: 'read_file',
    description: 'Read',
    schema: { type: 'object', properties: {} },
  },
];

test('anthropic: messages, raw replay, fallbacks and stop reasons', async () => {
  const seen = [];
  const reply = {
    content: [
      { type: 'thinking', thinking: '' },
      { type: 'text', text: 'Reading' },
      { type: 'tool_use', id: 't2', name: 'read_file', input: { path: 'b' } },
    ],
    stop_reason: 'tool_use',
  };
  const record = (kind) => async (p) => {
    seen.push([kind, p]);
    return reply;
  };
  const client = {
    messages: { create: record('plain') },
    beta: {
      messages: { create: record('beta') },
    },
  };
  const p = anthropicProvider({ model: 'claude-opus-5-5', timeoutMs: 1000 }, { client });
  const res = await p.complete({ system: 'sys', messages: history, tools });
  assert.deepEqual(res.toolCalls, [{ id: 't2', name: 'read_file', input: { path: 'b' } }]);
  assert.equal(res.stop, 'tool_use');
  const [kind, params] = seen[0];
  assert.equal(kind, 'beta');
  assert.equal(params.fallbacks, 'default');
  assert.deepEqual(params.betas, ['server-side-fallback-2026-07-01']);
  assert.equal(params.output_config.effort, 'high');
  assert.equal(params.tools[0].input_schema.type, 'object');
  assert.deepEqual(params.messages[2].content[0], {
    type: 'tool_result',
    tool_use_id: 't1',
    content: 'nope',
    is_error: true,
  });

  // The assistant turn is replayed exactly, thinking blocks included.
  const replay = toAnthropicMessages([{ role: 'assistant', raw: res.raw }]);
  assert.deepEqual(replay[0].content, reply.content);

  const proxied = anthropicProvider({ model: 'claude-opus-5-5', baseURL: 'https://proxy', timeoutMs: 1 }, { client });
  await proxied.complete({
    system: 's',
    messages: [{ role: 'user', text: 'x' }],
  });
  assert.equal(seen[1][0], 'plain', 'no server-side fallback through a custom base URL');
});

test('openai-compatible: messages, tool calls and missing tool support', async () => {
  const msgs = toOpenAIMessages('sys', history);
  assert.equal(msgs[0].role, 'system');
  assert.equal(msgs[2].tool_calls[0].function.arguments, '{"path":"a"}');
  assert.deepEqual(msgs[3], {
    role: 'tool',
    tool_call_id: 't1',
    content: 'nope',
  });

  let request;
  const ok = async (url, init) => {
    request = { url, body: JSON.parse(init.body), headers: init.headers };
    return new Response(
      JSON.stringify({
        choices: [
          {
            finish_reason: 'tool_calls',
            message: {
              content: null,
              tool_calls: [
                {
                  id: 'c1',
                  function: { name: 'read_file', arguments: '{"path":"x"' },
                },
              ],
            },
          },
        ],
      }),
    );
  };
  const p = openAIProvider({ model: 'qwen', baseURL: 'http://ollama:11434/v1/', timeoutMs: 1000 }, { fetch: ok });
  const res = await p.complete({
    system: 's',
    messages: [{ role: 'user', text: 'x' }],
    tools,
  });
  assert.equal(request.url, 'http://ollama:11434/v1/chat/completions');
  assert.equal(request.headers.Authorization, undefined, 'local servers need no key');
  assert.equal(request.body.tools[0].function.name, 'read_file');
  assert.equal(res.stop, 'tool_use');
  assert.ok(res.toolCalls[0].input.__invalid_json__, 'broken arguments reach the tool as invalid input');

  const noTools = async () => new Response('{"error":"model does not support tools"}', { status: 400 });
  const q = openAIProvider({ model: 'tiny', baseURL: 'http://x/v1', timeoutMs: 1000 }, { fetch: noTools });
  await assert.rejects(q.complete({ system: 's', messages: [], tools }), ToolsUnsupportedError);
  assert.equal(q.supportsTools, false);
});
