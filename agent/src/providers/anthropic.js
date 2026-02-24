// Claude through the official SDK and the Messages API.
import Anthropic from '@anthropic-ai/sdk';

export const DEFAULT_ANTHROPIC_MODEL = 'claude-opus-5-5';
const MAX_TOKENS = 16000;

// Models that take the server-side refusal fallback ("default" routing).
const FALLBACK_MODELS = new Set(['claude-opus-5-5', 'claude-opus-5', 'claude-fable-5-1', 'claude-sonnet-5-5']);
// Models whose effort is worth setting explicitly for agentic coding.
const EFFORT_MODELS = /^claude-(opus|sonnet|fable|haiku)-5/;

const STOPS = {
  end_turn: 'end',
  tool_use: 'tool_use',
  max_tokens: 'max_tokens',
  refusal: 'refusal',
};

/** Neutral messages to Messages API params; assistant turns replay raw content unchanged. */
export function toAnthropicMessages(messages) {
  return messages.map((m) => {
    if (m.role === 'user') return { role: 'user', content: m.text };
    if (m.role === 'assistant') {
      if (m.raw?.provider === 'anthropic') return { role: 'assistant', content: m.raw.content };
      const content = [];
      if (m.text) content.push({ type: 'text', text: m.text });
      for (const c of m.toolCalls ?? [])
        content.push({
          type: 'tool_use',
          id: c.id,
          name: c.name,
          input: c.input,
        });
      return { role: 'assistant', content };
    }
    return {
      role: 'user',
      content: m.results.map((r) => ({
        type: 'tool_result',
        tool_use_id: r.id,
        content: r.output,
        ...(r.isError ? { is_error: true } : {}),
      })),
    };
  });
}

/**
 * @param {{ model: string, apiKey?: string, baseURL?: string, effort?: string, timeoutMs: number }} ai
 * @param {{ client?: object }} deps a fake client in tests
 */
export function anthropicProvider(ai, deps = {}) {
  const model = ai.model || DEFAULT_ANTHROPIC_MODEL;
  const client =
    deps.client ??
    new Anthropic({
      ...(ai.apiKey ? { apiKey: ai.apiKey } : {}),
      ...(ai.baseURL ? { baseURL: ai.baseURL } : {}),
      timeout: ai.timeoutMs,
      maxRetries: 2,
    });
  // Fallbacks are a Claude API feature; a custom baseURL may be a proxy.
  const fallbacks = !ai.baseURL && FALLBACK_MODELS.has(model);
  const effort = ai.effort || (EFFORT_MODELS.test(model) ? 'high' : '');

  return {
    name: 'anthropic',
    model,
    supportsTools: true,
    async complete({ system, messages, tools, schema, signal }) {
      const outputConfig = {
        ...(effort ? { effort } : {}),
        ...(schema ? { format: { type: 'json_schema', schema } } : {}),
      };
      const params = {
        model,
        max_tokens: MAX_TOKENS,
        system,
        messages: toAnthropicMessages(messages),
        ...(tools?.length
          ? {
              tools: tools.map((t) => ({
                name: t.name,
                description: t.description,
                input_schema: t.schema,
              })),
            }
          : {}),
        ...(Object.keys(outputConfig).length ? { output_config: outputConfig } : {}),
      };
      const res = fallbacks
        ? await client.beta.messages.create(
            {
              ...params,
              betas: ['server-side-fallback-2026-07-01'],
              fallbacks: 'default',
            },
            { signal },
          )
        : await client.messages.create(params, { signal });
      return {
        text: res.content
          .filter((b) => b.type === 'text')
          .map((b) => b.text)
          .join('\n')
          .trim(),
        toolCalls: res.content
          .filter((b) => b.type === 'tool_use')
          .map((b) => ({ id: b.id, name: b.name, input: b.input })),
        stop: STOPS[res.stop_reason] ?? 'end',
        refusal: res.stop_reason === 'refusal' ? res.stop_details : undefined,
        raw: { provider: 'anthropic', content: res.content },
      };
    },
  };
}
