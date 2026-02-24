import fs from 'node:fs/promises';
import { relative, resolveInside } from './paths.js';
import { object } from './schema.js';
import { walk } from './walk.js';

const MAX_MATCHES = 100;
const MAX_FILE = 512 * 1024;

export function searchTool(root) {
  return {
    name: 'search',
    description: 'Search file contents with a JavaScript regular expression; returns path:line: text for each match.',
    schema: object(
      {
        pattern: { type: 'string', description: 'Regular expression.' },
        path: {
          type: 'string',
          description: 'Directory to search, relative to the root; default ".".',
        },
      },
      ['pattern'],
    ),
    async run({ pattern, path: dir = '.' }) {
      if (pattern.length > 500) throw new Error('pattern is too long');
      const re = new RegExp(pattern);
      const abs = await resolveInside(root, dir);
      const out = [];
      for await (const file of walk(abs)) {
        const stat = await fs.stat(file);
        if (stat.size > MAX_FILE) continue;
        const lines = (await fs.readFile(file, 'utf8')).split('\n');
        for (const [i, line] of lines.entries()) {
          if (!re.test(line)) continue;
          out.push(`${relative(root, file)}:${i + 1}: ${line.slice(0, 200)}`);
          if (out.length >= MAX_MATCHES) return `${out.join('\n')}\n… stopped after ${MAX_MATCHES} matches`;
        }
      }
      return out.length ? out.join('\n') : 'no matches';
    },
  };
}
