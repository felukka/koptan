// The tool set an agent run gets, with input checks and error capture.
import { commandTool } from './command.js';
import { fileTools } from './fs.js';
import { checkInput } from './schema.js';
import { searchTool } from './search.js';

const MAX_RESULT = 30 * 1024;

/**
 * @param {{ root: string, allowCommands: boolean }} opts root must be a real path
 */
export function createTools({ root, allowCommands }) {
  const list = [...fileTools(root), searchTool(root), ...(allowCommands ? [commandTool(root)] : [])];
  const byName = new Map(list.map((t) => [t.name, t]));
  return {
    /** Provider-neutral definitions: name, description, JSON schema. */
    definitions: list.map(({ name, description, schema }) => ({
      name,
      description,
      schema,
    })),
    /** Runs a tool; failures come back as an error result, never a throw. */
    async run(name, input) {
      const tool = byName.get(name);
      if (!tool) return { output: `unknown tool "${name}"`, isError: true };
      const problem = checkInput(tool.schema, input);
      if (problem) return { output: `invalid input: ${problem}`, isError: true };
      try {
        const output = String(await tool.run(input));
        return {
          output: output.length > MAX_RESULT ? `${output.slice(0, MAX_RESULT)}\n… truncated` : output,
          isError: false,
        };
      } catch (e) {
        return { output: e.message, isError: true };
      }
    },
    /** Applies a write or delete outside a tool call (plan mode). */
    apply: (name, input) => byName.get(name).run(input),
  };
}
