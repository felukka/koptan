// Plan mode, for models without tool calling: the model sees the files and
// returns a JSON edit plan, which is validated and then applied with the
// same confined file tools.
import fs from 'node:fs/promises';
import { relative } from './tools/paths.js';
import { walk } from './tools/walk.js';

const MAX_CONTEXT = 60 * 1024;
const MAX_FILE = 16 * 1024;

export const PLAN_SCHEMA = {
  type: 'object',
  properties: {
    summary: { type: 'string' },
    changes: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          path: { type: 'string' },
          action: { type: 'string', enum: ['write', 'delete'] },
          content: { type: 'string' },
        },
        required: ['path', 'action', 'content'],
        additionalProperties: false,
      },
    },
  },
  required: ['summary', 'changes'],
  additionalProperties: false,
};

/** The repository's small text files, bounded, for the prompt. */
async function snapshot(root) {
  let out = '';
  for await (const file of walk(root)) {
    const stat = await fs.stat(file);
    if (stat.size > MAX_FILE) continue;
    const text = await fs.readFile(file, 'utf8');
    if (text.includes('\0')) continue;
    const block = `--- ${relative(root, file)}\n${text}\n`;
    if (out.length + block.length > MAX_CONTEXT) break;
    out += block;
  }
  return out || '(empty repository)';
}

/** Parses and checks a plan; throws on anything malformed. */
export function parsePlan(text) {
  const json = text.trim().replace(/^```(?:json)?\s*|\s*```$/g, '');
  const plan = JSON.parse(json);
  if (typeof plan?.summary !== 'string' || !Array.isArray(plan.changes))
    throw new Error('plan must have summary and changes');
  for (const c of plan.changes) {
    if (typeof c?.path !== 'string' || !['write', 'delete'].includes(c.action))
      throw new Error('every change needs a path and an action');
    if (c.action === 'write' && typeof c.content !== 'string') throw new Error(`write to ${c.path} has no content`);
  }
  return plan;
}

export async function runPlan({ provider, tools, root, system, prompt, onEvent, signal }) {
  onEvent({
    type: 'status',
    text: 'The model has no tool calling; asking for an edit plan',
  });
  const files = await snapshot(root);
  const res = await provider.complete({
    system,
    messages: [
      {
        role: 'user',
        text: `${prompt}\n\nThe repository's files follow. Reply with only a JSON object {"summary": string, "changes": [{"path", "action": "write"|"delete", "content"}]} where content is the full new file (empty for delete).\n\n${files}`,
      },
    ],
    schema: PLAN_SCHEMA,
    signal,
  });
  const plan = parsePlan(res.text);
  for (const change of plan.changes) {
    const name = change.action === 'write' ? 'write_file' : 'delete_file';
    const input = change.action === 'write' ? { path: change.path, content: change.content } : { path: change.path };
    onEvent({ type: 'tool', name, input: { path: change.path } });
    const result = await tools.run(name, input);
    onEvent({
      type: 'tool_result',
      name,
      output: result.output,
      isError: result.isError,
    });
  }
  return { summary: plan.summary };
}
