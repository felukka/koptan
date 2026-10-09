// run_command exists only when the SelfService allows commands. It runs in
// the workspace with a minimal environment: no git or AI credentials.
import { execFile } from 'node:child_process';
import { object } from './schema.js';

const TIMEOUT_MS = 120_000;
const MAX_OUTPUT = 8 * 1024;

const tail = (s) => (s.length > MAX_OUTPUT ? `…${s.slice(-MAX_OUTPUT)}` : s);

export function commandTool(root) {
  return {
    name: 'run_command',
    description:
      'Run a shell command in the repository root (for example to run tests or a formatter). Times out after 2 minutes.',
    schema: object({
      command: { type: 'string', description: 'The shell command.' },
    }),
    run({ command }) {
      return new Promise((resolve) => {
        execFile(
          'sh',
          ['-c', command],
          {
            cwd: root,
            timeout: TIMEOUT_MS,
            maxBuffer: 4 * 1024 * 1024,
            env: {
              PATH: process.env.PATH,
              HOME: process.env.HOME ?? '/tmp',
              LANG: 'C.UTF-8',
              CI: 'true',
            },
          },
          (err, stdout, stderr) => {
            const code = err ? (err.killed ? 'timeout' : (err.code ?? 1)) : 0;
            resolve(`exit ${code}\n${tail(`${stdout}${stderr}`)}`);
          },
        );
      });
    },
  };
}
