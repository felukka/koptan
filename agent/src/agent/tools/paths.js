// Every tool path goes through resolveInside: relative to the workspace,
// never into .git, never out through `..` or a symlink.
import fs from 'node:fs/promises';
import path from 'node:path';

export class PathError extends Error {}

const inside = (root, p) => p === root || p.startsWith(root + path.sep);

/** The real path of p, or of its nearest existing parent plus the rest. */
async function realish(p) {
  const rest = [];
  let cur = p;
  for (;;) {
    try {
      return path.join(await fs.realpath(cur), ...rest.reverse());
    } catch (e) {
      if (e.code !== 'ENOENT') throw e;
      const parent = path.dirname(cur);
      if (parent === cur) throw e;
      rest.push(path.basename(cur));
      cur = parent;
    }
  }
}

/**
 * Resolves a model-supplied path inside root (a real path).
 * @returns {Promise<string>} the absolute path
 */
export async function resolveInside(root, rel) {
  if (typeof rel !== 'string' || rel.includes('\0')) throw new PathError('path must be a string');
  const target = path.resolve(root, rel || '.');
  if (!inside(root, target)) throw new PathError(`${rel} is outside the repository`);
  const segments = path.relative(root, target).split(path.sep);
  if (segments.includes('.git')) throw new PathError('the .git directory is off limits');
  const real = await realish(target);
  if (!inside(root, real)) throw new PathError(`${rel} resolves outside the repository`);
  return target;
}

/** The workspace-relative form of an absolute path, with / separators. */
export const relative = (root, abs) => path.relative(root, abs).split(path.sep).join('/') || '.';
