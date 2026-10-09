// The pluggable provider layer. Every provider implements
//
//   complete({ system, messages, tools?, schema?, signal? }) ->
//     { text, toolCalls: [{ id, name, input }], stop, raw }
//
// over provider-neutral messages:
//   { role: 'user', text }
//   { role: 'assistant', text, toolCalls, raw }   raw is replayed as-is
//   { role: 'tool', results: [{ id, name, output, isError }] }
//
// stop is 'end', 'tool_use', 'max_tokens' or 'refusal'.
import { anthropicProvider } from './anthropic.js';
import { fakeProvider } from './fake.js';
import { openAIProvider } from './openai.js';

export { ToolsUnsupportedError } from './errors.js';

export function createProvider(ai) {
  switch (ai.provider) {
    case 'anthropic':
      return anthropicProvider(ai);
    case 'openai-compatible':
      return openAIProvider(ai);
    case 'fake':
      return fakeProvider([]);
    default:
      throw new Error(`unknown AI provider "${ai.provider}"`);
  }
}
