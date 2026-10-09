// The tool loop: ask the model, run the tools it calls, feed the results
// back, until it answers without tool calls.
import { ToolsUnsupportedError } from '../providers/errors.js';
import { runPlan } from './plan.js';

const PREVIEW = 400;
const preview = (s) => (s.length > PREVIEW ? `${s.slice(0, PREVIEW)}…` : s);

/** Shortens tool input for progress events (file contents can be large). */
function describeInput(input) {
  const out = {};
  for (const [k, v] of Object.entries(input ?? {})) out[k] = typeof v === 'string' ? preview(v) : v;
  return out;
}

/**
 * @param {{ provider: object, tools: object, root: string, system: string, prompt: string,
 *   maxSteps: number, onEvent: (e: object) => void, signal?: AbortSignal }} opts
 * @returns {Promise<{ summary: string }>}
 */
export async function runAgent(opts) {
  const { provider, tools, system, prompt, maxSteps, onEvent, signal } = opts;
  if (!provider.supportsTools) return runPlan(opts);
  const messages = [{ role: 'user', text: prompt }];

  for (let step = 1; step <= maxSteps; step++) {
    let res;
    try {
      res = await provider.complete({
        system,
        messages,
        tools: tools.definitions,
        signal,
      });
    } catch (e) {
      if (e instanceof ToolsUnsupportedError) return runPlan(opts);
      throw e;
    }
    if (res.text) onEvent({ type: 'message', text: res.text });
    if (res.stop === 'refusal') throw new Error('the model declined this request');
    if (!res.toolCalls.length) {
      if (res.stop === 'max_tokens') throw new Error('the model ran out of output tokens');
      return { summary: res.text };
    }
    // A tool input cut off at max_tokens may parse but be incomplete.
    if (res.stop === 'max_tokens') throw new Error('a tool call was cut off (max_tokens)');

    messages.push({
      role: 'assistant',
      text: res.text,
      toolCalls: res.toolCalls,
      raw: res.raw,
    });
    const results = [];
    for (const call of res.toolCalls) {
      onEvent({
        type: 'tool',
        name: call.name,
        input: describeInput(call.input),
      });
      const result = await tools.run(call.name, call.input);
      onEvent({
        type: 'tool_result',
        name: call.name,
        output: preview(result.output),
        isError: result.isError,
      });
      results.push({ id: call.id, name: call.name, ...result });
    }
    messages.push({ role: 'tool', results });
  }
  throw new Error(`stopped after ${maxSteps} steps without finishing`);
}
