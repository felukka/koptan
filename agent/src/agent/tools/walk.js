import fs from 'node:fs/promises';
import path from 'node:path';

/** Directories no tool lists or searches. */
export const SKIPPED = new Set([
  '.git',
  'node_modules',
  '.venv',
  'venv',
  '__pycache__',
  'target',
  'dist',
  'build',
  'vendor',
]);

/**
 * Yields the files under dir (absolute paths), depth-first and sorted,
 * skipping SKIPPED directories and symlinks.
 */
export async function* walk(dir, maxDepth = 12) {
  let entries;
  try {
    entries = await fs.readdir(dir, { withFileTypes: true });
  } catch {
    return;
  }
  entries.sort((a, b) => a.name.localeCompare(b.name));
  for (const e of entries) {
    const full = path.join(dir, e.name);
    if (e.isDirectory() && !SKIPPED.has(e.name) && maxDepth > 0) {
      yield* walk(full, maxDepth - 1);
    } else if (e.isFile()) {
      yield full;
    }
  }
}
