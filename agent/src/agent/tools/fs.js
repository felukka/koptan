// File tools. Every path is resolved inside the workspace first.
import fs from 'node:fs/promises';
import path from 'node:path';
import { relative, resolveInside } from './paths.js';
import { object } from './schema.js';
import { walk } from './walk.js';

const MAX_READ = 200 * 1024;
const MAX_WRITE = 1024 * 1024;
const MAX_LIST = 400;

const str = (description) => ({ type: 'string', description });

export function fileTools(root) {
  return [
    {
      name: 'list_files',
      description:
        'List the files under a directory of the repository (recursive; skips .git, node_modules and build output).',
      schema: object(
        {
          path: str('Directory relative to the repository root; "." for the root.'),
        },
        [],
      ),
      async run({ path: dir = '.' }) {
        const abs = await resolveInside(root, dir);
        const out = [];
        for await (const file of walk(abs)) {
          out.push(relative(root, file));
          if (out.length >= MAX_LIST) {
            out.push(`… stopped after ${MAX_LIST} files`);
            break;
          }
        }
        return out.length ? out.join('\n') : '(no files)';
      },
    },
    {
      name: 'read_file',
      description: 'Read a text file of the repository.',
      schema: object({
        path: str('File path relative to the repository root.'),
      }),
      async run({ path: file }) {
        const abs = await resolveInside(root, file);
        const stat = await fs.stat(abs);
        if (stat.size > MAX_READ) throw new Error(`${file} is ${stat.size} bytes; the limit is ${MAX_READ}`);
        return await fs.readFile(abs, 'utf8');
      },
    },
    {
      name: 'write_file',
      description: 'Create or overwrite a file with the given content. Parent directories are created.',
      schema: object({
        path: str('File path relative to the repository root.'),
        content: str('The full new file content.'),
      }),
      async run({ path: file, content }) {
        if (Buffer.byteLength(content) > MAX_WRITE) throw new Error(`content is larger than ${MAX_WRITE} bytes`);
        const abs = await resolveInside(root, file);
        await fs.mkdir(path.dirname(abs), { recursive: true });
        await fs.writeFile(abs, content);
        return `wrote ${relative(root, abs)} (${Buffer.byteLength(content)} bytes)`;
      },
    },
    {
      name: 'edit_file',
      description:
        'Replace one exact occurrence of old_text with new_text in a file. old_text must appear exactly once.',
      schema: object({
        path: str('File path relative to the repository root.'),
        old_text: str('Text to replace, copied exactly, unique in the file.'),
        new_text: str('Replacement text.'),
      }),
      async run({ path: file, old_text: oldText, new_text: newText }) {
        const abs = await resolveInside(root, file);
        const text = await fs.readFile(abs, 'utf8');
        const count = oldText ? text.split(oldText).length - 1 : 0;
        if (count !== 1) throw new Error(`old_text appears ${count} times in ${file}; it must appear exactly once`);
        await fs.writeFile(
          abs,
          text.replace(oldText, () => newText),
        );
        return `edited ${relative(root, abs)}`;
      },
    },
    {
      name: 'delete_file',
      description: 'Delete a file of the repository.',
      schema: object({
        path: str('File path relative to the repository root.'),
      }),
      async run({ path: file }) {
        const abs = await resolveInside(root, file);
        if (abs === root) throw new Error('cannot delete the repository root');
        await fs.rm(abs);
        return `deleted ${relative(root, abs)}`;
      },
    },
  ];
}
