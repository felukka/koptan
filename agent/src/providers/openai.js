// Any /v1/chat/completions server: OpenAI, Azure OpenAI, Gemini's OpenAI
// endpoint, OpenRouter, LiteLLM, Ollama, vLLM, LM Studio, llama.cpp.
import { ToolsUnsupportedError } from './errors.js';

const STOPS = {
  stop: 'end',
  tool_calls: 'tool_use',
  length: 'max_tokens',
  content_filter: 'refusal',
};

/** Neutral messages to chat-completions messages. */
export function toOpenAIMessages(system, messages) {
  const out = [{ role: 'system', content: system }];
  for (const m of messages) {
    if (m.role === 'user') out.push({ role: 'user', content: m.text });
    else if (m.role === 'assistant') {
      out.push({
        role: 'assistant',
        content: m.text || null,
        ...(m.toolCalls?.length
          ? {
              tool_calls: m.toolCalls.map((c) => ({
                id: c.id,
                type: 'function',
                function: { name: c.name, arguments: JSON.stringify(c.input) },
              })),
            }
          : {}),
      });
    } else {
      for (const r of m.results) out.push({ role: 'tool', tool_call_id: r.id, content: r.output });
    }
  }
  return out;
}

/** Arguments arrive as a JSON string; invalid JSON becomes an input the tool rejects. */
function parseArguments(text) {
  try {
    return JSON.parse(text || '{}');
  } catch {
    return { __invalid_json__: String(text) };
  }
}

/**
 * @param {{ model: string, baseURL: string, apiKey?: string, timeoutMs: number, tools: boolean }} ai
 * @param {{ fetch?: typeof fetch }} deps
 */
export function openAIProvider(ai, deps = {}) {
  const doFetch = deps.fetch ?? fetch;
  const url = `${ai.baseURL.replace(/\/+$/, '')}/chat/completions`;
  const provider = {
    name: 'openai-compatible',
    model: ai.model,
    supportsTools: ai.tools !== false,
    async complete({ system, messages, tools, schema, signal }) {
      const body = {
        model: ai.model,
        messages: toOpenAIMessages(system, messages),
        ...(tools?.length
          ? {
              tools: tools.map((t) => ({
                type: 'function',
                function: {
                  name: t.name,
                  description: t.description,
                  parameters: t.schema,
                },
              })),
            }
          : {}),
        ...(schema
          ? {
              response_format: {
                type: 'json_schema',
                json_schema: { name: 'result', schema, strict: true },
              },
            }
          : {}),
      };
      const res = await doFetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(ai.apiKey ? { Authorization: `Bearer ${ai.apiKey}` } : {}),
        },
        body: JSON.stringify(body),
        signal: signal ?? AbortSignal.timeout(ai.timeoutMs),
      });
      const text = await res.text();
      if (!res.ok) {
        if (res.status === 400 && tools?.length && /tool|function/i.test(text)) {
          provider.supportsTools = false;
          throw new ToolsUnsupportedError(`the model rejected tool calling: ${text.slice(0, 200)}`);
        }
        throw new Error(`${ai.model} answered ${res.status}: ${text.slice(0, 300)}`);
      }
      const choice = JSON.parse(text).choices?.[0];
      if (!choice) throw new Error('the model returned no choices');
      const calls = choice.message?.tool_calls ?? [];
      return {
        text: (choice.message?.content ?? '').trim(),
        toolCalls: calls.map((c) => ({
          id: c.id,
          name: c.function.name,
          input: parseArguments(c.function.arguments),
        })),
        stop: calls.length ? 'tool_use' : (STOPS[choice.finish_reason] ?? 'end'),
        raw: { provider: 'openai-compatible' },
      };
    },
  };
  return provider;
}
