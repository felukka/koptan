import type {
  CreateSelfServiceRequest,
  SelfService,
} from '@internal/plugin-koptan-common';
import { createWithSecrets, type KoptanClient, type SecretSpec } from '../k8s';
import { cleanMetadata, validateName } from '../pipelines';

const OWNER = /^[A-Za-z0-9_.-]+(\/[A-Za-z0-9_.-]+)*$/;
const REPO_NAME = /^[A-Za-z0-9_.-]{1,100}$/;
const BRANCH = /^[A-Za-z0-9._/-]{1,200}$/;

const isHttps = (url: string) => {
  try {
    const u = new URL(url);
    return (
      u.protocol === 'https:' && !!u.hostname && !u.username && !u.password
    );
  } catch {
    return false;
  }
};

const isHttp = (url: string) => {
  try {
    return ['http:', 'https:'].includes(new URL(url).protocol);
  } catch {
    return false;
  }
};

export const redactSelfService = (raw: Record<string, unknown>) =>
  ({ ...raw, metadata: cleanMetadata(raw) }) as SelfService;

export function validateCreateSelfService(
  req: CreateSelfServiceRequest,
): string | undefined {
  const named = validateName(req);
  if (named) return named;
  const repo = req.repo;
  if (!repo?.token) return 'a git token is required';
  if (repo.mode === 'existing') {
    if (!isHttps(repo.url ?? ''))
      return 'the repository URL must be https without credentials';
  } else if (repo.mode === 'create') {
    if (!['github', 'gitlab'].includes(repo.provider))
      return 'provider must be github or gitlab';
    if (repo.owner && !OWNER.test(repo.owner)) return 'owner is not valid';
    if (repo.repoName && !REPO_NAME.test(repo.repoName))
      return 'repository name is not valid';
    if (repo.baseURL && !isHttps(repo.baseURL)) return 'baseURL must be https';
  } else {
    return 'repo.mode must be existing or create';
  }
  if (req.branch && (!BRANCH.test(req.branch) || req.branch.includes('..')))
    return 'branch is not valid';
  const ai = req.ai;
  if (!ai || !['anthropic', 'openai-compatible'].includes(ai.provider))
    return 'ai.provider must be anthropic or openai-compatible';
  if (!ai.model?.trim()) return 'ai.model is required';
  if (ai.provider === 'openai-compatible' && !isHttp(ai.baseURL ?? ''))
    return 'an OpenAI-compatible provider needs an http(s) baseURL';
  if (
    req.port !== undefined &&
    (!Number.isInteger(req.port) || req.port < 1 || req.port > 65535)
  )
    return 'port must be 1-65535';
  return undefined;
}

/**
 * Creates the SelfService. The git token goes to Secret `<name>-git` and
 * the AI key to `<name>-ai`, both owned by the SelfService.
 */
export async function createSelfService(
  client: KoptanClient,
  req: CreateSelfServiceRequest,
  createdBy: string,
): Promise<SelfService> {
  const namespace = req.namespace ?? 'default';
  const gitRef = { name: `${req.name}-git`, key: 'token' };
  const secrets: SecretSpec[] = [
    { name: gitRef.name, data: { token: req.repo.token } },
  ];
  if (req.ai.apiKey) {
    secrets.push({ name: `${req.name}-ai`, data: { apiKey: req.ai.apiKey } });
  }
  const r = req.repo;
  const repo =
    r.mode === 'existing'
      ? { existing: { url: r.url, secretRef: gitRef } }
      : {
          create: {
            provider: r.provider,
            ...(r.owner ? { owner: r.owner } : {}),
            ...(r.repoName ? { name: r.repoName } : {}),
            private: r.private ?? true,
            ...(r.baseURL ? { baseURL: r.baseURL } : {}),
            tokenSecretRef: gitRef,
          },
        };
  const created = await createWithSecrets(
    client,
    'SelfService',
    namespace,
    secrets,
    {
      metadata: {
        name: req.name,
        namespace,
        annotations: { 'koptan.felukka.org/created-by': createdBy },
      },
      spec: {
        repo,
        ...(req.branch ? { branch: req.branch } : {}),
        ai: {
          provider: req.ai.provider,
          model: req.ai.model.trim(),
          ...(req.ai.baseURL ? { baseURL: req.ai.baseURL } : {}),
          ...(req.ai.apiKey
            ? { apiKeySecretRef: { name: `${req.name}-ai`, key: 'apiKey' } }
            : {}),
        },
        ...(req.port ? { service: { port: req.port } } : {}),
        ...(req.allowCommands ? { allowCommands: true } : {}),
      },
    },
  );
  return redactSelfService(created);
}
