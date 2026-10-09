import { execFileSync } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

export async function tempDir(prefix = 'koptan-agent-test-') {
  return fs.realpath(await fs.mkdtemp(path.join(os.tmpdir(), prefix)));
}

/** A bare repository to push to, and a Repo-ready author. */
export async function bareRepo() {
  const dir = await tempDir('koptan-bare-');
  execFileSync('git', ['init', '-q', '--bare', '-b', 'main', dir]);
  return dir;
}

export const author = { name: 'Test', email: 'test@example.com' };

export const gitIn = (dir, ...args) =>
  execFileSync('git', ['-C', dir, ...args])
    .toString()
    .trim();
