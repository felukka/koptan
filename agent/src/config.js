// Configuration from the environment (set by the operator on the session
// pod) and command-line flags, validated once at startup.

const PROVIDERS = ['anthropic', 'openai-compatible', 'fake'];

/** Thrown for configuration the agent cannot run with. */
export class ConfigError extends Error {}

const bool = (v, fallback) =>
  v === undefined || v === '' ? fallback : ['1', 'true', 'yes'].includes(String(v).toLowerCase());

const int = (v, fallback, name) => {
  if (v === undefined || v === '') return fallback;
  const n = Number(v);
  if (!Number.isInteger(n) || n <= 0) throw new ConfigError(`${name} must be a positive integer`);
  return n;
};

/**
 * Builds the configuration. `requireServer` adds the checks only `serve`
 * needs (an API token and a remote repository).
 * @param {Record<string, string | undefined>} env
 * @param {Record<string, string | boolean>} flags
 */
export function loadConfig(env, flags = {}, { requireServer = false } = {}) {
  const pick = (flag, key, fallback) => flags[flag] ?? env[key] ?? fallback;
  const cfg = {
    repo: pick('repo', 'KOPTAN_AGENT_REPO', ''),
    branch: pick('branch', 'KOPTAN_AGENT_BRANCH', 'main'),
    workspace: pick('workspace', 'KOPTAN_AGENT_WORKSPACE', '/workspace/repo'),
    port: int(pick('port', 'KOPTAN_AGENT_PORT'), 8080, 'port'),
    apiToken: env.KOPTAN_AGENT_TOKEN ?? '',
    gitToken: env.KOPTAN_GIT_TOKEN ?? env.GIT_TOKEN ?? '',
    allowCommands: bool(pick('allow-commands', 'KOPTAN_AGENT_ALLOW_COMMANDS'), false),
    maxSteps: int(pick('max-steps', 'KOPTAN_AGENT_MAX_STEPS'), 40, 'max-steps'),
    service: env.KOPTAN_AGENT_SERVICE ?? '',
    appPort: int(env.KOPTAN_AGENT_APP_PORT, 8080, 'KOPTAN_AGENT_APP_PORT'),
    author: {
      name: env.GIT_AUTHOR_NAME || 'Koptan Agent',
      email: env.GIT_AUTHOR_EMAIL || 'agent@koptan.felukka.org',
    },
    ai: {
      provider: pick('provider', 'KOPTAN_AI_PROVIDER', ''),
      model: pick('model', 'KOPTAN_AI_MODEL', ''),
      baseURL: env.KOPTAN_AI_BASE_URL ?? '',
      apiKey: env.KOPTAN_AI_API_KEY ?? '',
      effort: env.KOPTAN_AI_EFFORT ?? '',
      timeoutMs: int(env.KOPTAN_AI_TIMEOUT_MS, 600_000, 'KOPTAN_AI_TIMEOUT_MS'),
      tools: bool(env.KOPTAN_AI_TOOLS, true),
    },
  };
  validate(cfg, requireServer);
  return cfg;
}

function validate(cfg, requireServer) {
  if (!PROVIDERS.includes(cfg.ai.provider)) {
    throw new ConfigError(`KOPTAN_AI_PROVIDER must be one of ${PROVIDERS.join(', ')}`);
  }
  if (!cfg.ai.model && cfg.ai.provider !== 'fake') {
    throw new ConfigError('KOPTAN_AI_MODEL is required');
  }
  if (cfg.ai.provider === 'openai-compatible' && !cfg.ai.baseURL) {
    throw new ConfigError('KOPTAN_AI_BASE_URL is required for openai-compatible');
  }
  if (!/^[A-Za-z0-9._/-]{1,200}$/.test(cfg.branch) || cfg.branch.includes('..')) {
    throw new ConfigError('the branch name is not valid');
  }
  if (!requireServer) return;
  if (cfg.apiToken.length < 16) {
    throw new ConfigError('KOPTAN_AGENT_TOKEN must be set (16+ characters)');
  }
  validateRepoURL(cfg.repo);
}

/** The agent pushes with a token over https, so only https remotes. */
export function validateRepoURL(url) {
  let u;
  try {
    u = new URL(url);
  } catch {
    throw new ConfigError('KOPTAN_AGENT_REPO must be an https URL');
  }
  if (u.protocol !== 'https:' || !u.hostname || u.username || u.password) {
    throw new ConfigError('KOPTAN_AGENT_REPO must be an https URL without credentials');
  }
}
