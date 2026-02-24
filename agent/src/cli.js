// koptan-agent serve | run "<prompt>" | help
import { parseArgs } from 'node:util';
import { ConfigError, loadConfig } from './config.js';
import { Repo } from './git.js';
import { addSecret, log } from './log.js';
import { createProvider } from './providers/index.js';
import { createServer } from './server.js';
import { Session } from './session.js';

const HELP = `koptan-agent: turns prompts into commits on a git repository.

Usage:
  koptan-agent serve              serve the HTTP API (POST /runs, GET /runs, GET /healthz)
  koptan-agent run "<prompt>"     run one prompt against the repository and exit

Flags (override the KOPTAN_* environment):
  --repo URL  --branch NAME  --workspace DIR  --port N  --provider NAME  --model ID
  --allow-commands  --max-steps N

Environment: KOPTAN_AGENT_REPO, KOPTAN_AGENT_BRANCH, KOPTAN_AGENT_TOKEN, KOPTAN_GIT_TOKEN,
  KOPTAN_AI_PROVIDER (anthropic | openai-compatible), KOPTAN_AI_MODEL, KOPTAN_AI_BASE_URL,
  KOPTAN_AI_API_KEY, KOPTAN_AI_EFFORT, KOPTAN_AI_TOOLS, KOPTAN_AGENT_ALLOW_COMMANDS.`;

const OPTIONS = {
  repo: { type: 'string' },
  branch: { type: 'string' },
  workspace: { type: 'string' },
  port: { type: 'string' },
  provider: { type: 'string' },
  model: { type: 'string' },
  'allow-commands': { type: 'boolean' },
  'max-steps': { type: 'string' },
  help: { type: 'boolean', short: 'h' },
};

function build(config) {
  for (const s of [config.apiToken, config.gitToken, config.ai.apiKey]) addSecret(s);
  const repo = new Repo({
    url: config.repo,
    branch: config.branch,
    dir: config.workspace,
    token: config.gitToken,
    author: config.author,
  });
  return {
    repo,
    session: new Session({ repo, provider: createProvider(config.ai), config }),
  };
}

async function serve(flags) {
  const config = loadConfig(process.env, flags, { requireServer: true });
  const { repo, session } = build(config);
  await repo.prepare();
  const seeded = await repo.seed(`# ${config.service || 'Koptan service'}\n\nBuilt and deployed by Koptan.\n`);
  if (seeded) log('info', 'seeded the empty repository', { commit: seeded });
  const server = createServer(session, config.apiToken);
  server.listen(config.port, () => log('info', 'listening', { port: config.port, branch: config.branch }));
  const stop = () => server.close(() => process.exit(0));
  process.on('SIGTERM', stop);
  process.on('SIGINT', stop);
}

async function runOnce(flags, prompt) {
  if (!prompt) throw new ConfigError('run needs a prompt');
  const config = loadConfig(process.env, flags);
  const { session } = build(config);
  const run = await session.run(prompt, (e) => process.stdout.write(`${JSON.stringify(e)}\n`));
  return run.status === 'succeeded' ? 0 : 1;
}

/** @returns {Promise<number>} the exit code */
export async function main(argv) {
  let parsed;
  try {
    parsed = parseArgs({
      args: argv,
      options: OPTIONS,
      allowPositionals: true,
    });
  } catch (e) {
    process.stderr.write(`${e.message}\n\n${HELP}\n`);
    return 2;
  }
  const { values: flags, positionals } = parsed;
  const [command, ...rest] = positionals;
  if (flags.help || !command || command === 'help') {
    process.stdout.write(`${HELP}\n`);
    return command || flags.help ? 0 : 2;
  }
  try {
    if (command === 'serve') {
      await serve(flags);
      return -1; // keeps running
    }
    if (command === 'run') return await runOnce(flags, rest.join(' '));
    process.stderr.write(`unknown command "${command}"\n\n${HELP}\n`);
    return 2;
  } catch (e) {
    process.stderr.write(`koptan-agent: ${e.message}\n`);
    return e instanceof ConfigError ? 2 : 1;
  }
}
