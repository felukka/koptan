// The working copy. The token reaches git only through GIT_ASKPASS and an
// environment variable, never in a URL, argument or file in the repository.
import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const ASKPASS = `#!/bin/sh
case "$1" in
  Username*) echo x-access-token ;;
  *) echo "$KOPTAN_GIT_TOKEN" ;;
esac
`;

export class Repo {
  /**
   * @param {{ url: string, branch: string, dir: string, token?: string,
   *   author: { name: string, email: string } }} opts
   */
  constructor({ url, branch, dir, token = '', author }) {
    Object.assign(this, { url, branch, dir, token, author });
  }

  async #env() {
    if (!this.askpass) {
      const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'koptan-git-'));
      this.askpass = path.join(dir, 'askpass.sh');
      await fs.writeFile(this.askpass, ASKPASS, { mode: 0o700 });
    }
    return {
      PATH: process.env.PATH,
      HOME: process.env.HOME ?? os.tmpdir(),
      GIT_ASKPASS: this.askpass,
      GIT_TERMINAL_PROMPT: '0',
      KOPTAN_GIT_TOKEN: this.token,
      GIT_AUTHOR_NAME: this.author.name,
      GIT_AUTHOR_EMAIL: this.author.email,
      GIT_COMMITTER_NAME: this.author.name,
      GIT_COMMITTER_EMAIL: this.author.email,
    };
  }

  /** Runs git; `--` keeps user-controlled values from being read as options. */
  async git(args, { cwd = this.dir, allowFail = false } = {}) {
    const env = await this.#env();
    return new Promise((resolve, reject) => {
      execFile('git', args, { cwd, env, maxBuffer: 16 * 1024 * 1024 }, (err, stdout, stderr) => {
        if (err && !allowFail) {
          reject(new Error(`git ${args[0]} failed: ${(stderr || err.message).trim()}`));
          return;
        }
        resolve({ ok: !err, stdout: stdout.trim(), stderr: stderr.trim() });
      });
    });
  }

  async #remoteHasBranch() {
    const { stdout } = await this.git(['ls-remote', '--heads', 'origin', `refs/heads/${this.branch}`]);
    return stdout !== '';
  }

  /** Clones on first use, then resets to the remote branch (or an empty one). */
  async prepare() {
    try {
      await fs.access(path.join(this.dir, '.git'));
    } catch {
      await fs.mkdir(path.dirname(this.dir), { recursive: true });
      await this.git(['clone', '--no-tags', '--', this.url, this.dir], {
        cwd: path.dirname(this.dir),
      });
    }
    if (await this.#remoteHasBranch()) {
      await this.git(['fetch', '--no-tags', 'origin', `+refs/heads/${this.branch}:refs/remotes/origin/${this.branch}`]);
      await this.git(['checkout', '-q', '-B', this.branch, `origin/${this.branch}`]);
      await this.git(['reset', '-q', '--hard', `origin/${this.branch}`]);
    } else {
      // Empty repository or a new branch: start it unborn or from HEAD.
      await this.git(['checkout', '-q', '-B', this.branch], {
        allowFail: true,
      });
      await this.git(['symbolic-ref', 'HEAD', `refs/heads/${this.branch}`]);
      await this.git(['reset', '-q', '--hard'], { allowFail: true });
    }
    await this.git(['clean', '-q', '-fdx']);
  }

  /** Gives an empty repository a first commit so the branch exists. */
  async seed(readme) {
    if (await this.#remoteHasBranch()) return null;
    await fs.writeFile(path.join(this.dir, 'README.md'), readme, { flag: 'wx' }).catch(() => undefined);
    return this.commitAndPush('Initialize repository\n\nCommitted by the Koptan agent.');
  }

  /** Commits every change and pushes; null when nothing changed. */
  async commitAndPush(message) {
    await this.git(['add', '-A']);
    const { stdout } = await this.git(['status', '--porcelain']);
    if (!stdout) return null;
    await this.git(['commit', '-q', '-m', message]);
    await this.git(['push', '-q', 'origin', `HEAD:refs/heads/${this.branch}`]);
    return (await this.git(['rev-parse', 'HEAD'])).stdout;
  }
}
