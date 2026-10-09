import type {
  CreatePipelineRequest,
  Pipeline,
} from '@internal/plugin-koptan-common';
import { createWithSecrets, type KoptanClient, type SecretSpec } from '../k8s';
import { redactService } from './redact';

/** A .dockerconfigjson for one registry. */
export function dockerConfigJson(
  registry: string,
  username: string,
  password: string,
): string {
  const auth = Buffer.from(`${username}:${password}`).toString('base64');
  return JSON.stringify({
    auths: { [registry]: { username, password, auth } },
  });
}

/**
 * Creates the Service; the operator derives its CI and CD. The git token is
 * stored as Secret `<name>-git` and registry credentials as dockerconfigjson
 * Secret `<name>-registry`, both owned by the Service.
 */
export async function createPipeline(
  client: KoptanClient,
  req: CreatePipelineRequest,
  createdBy: string,
): Promise<Pipeline> {
  const namespace = req.namespace ?? 'default';
  const gitSecret = `${req.name}-git`;
  const registrySecret = `${req.name}-registry`;
  const img = req.image;

  const secrets: SecretSpec[] = [];
  if (req.token) {
    secrets.push({ name: gitSecret, data: { token: req.token } });
  }
  if (img?.username && img.password && img.registry) {
    secrets.push({
      name: registrySecret,
      type: 'kubernetes.io/dockerconfigjson',
      data: {
        '.dockerconfigjson': dockerConfigJson(
          img.registry,
          img.username,
          img.password,
        ),
      },
    });
  }

  const withLogin = secrets.some((s) => s.name === registrySecret);
  const image = {
    ...(img?.registry ? { registry: img.registry } : {}),
    ...(img?.repo ? { repo: img.repo } : {}),
    ...(withLogin ? { credentialsSecret: registrySecret } : {}),
  };
  const service = await createWithSecrets(
    client,
    'Service',
    namespace,
    secrets,
    {
      metadata: {
        name: req.name,
        namespace,
        annotations: { 'koptan.felukka.org/created-by': createdBy },
      },
      spec: {
        source: {
          repo: req.repo,
          ...(req.revision ? { revision: req.revision } : {}),
          ...(req.token
            ? { secretRef: { name: gitSecret, key: 'token' } }
            : {}),
        },
        ...(req.env?.length ? { env: req.env } : {}),
        ...(Object.keys(image).length ? { image } : {}),
        ...(req.replicas !== undefined ? { replicas: req.replicas } : {}),
        ...(req.port !== undefined ? { port: req.port } : {}),
      },
    },
  );
  return { service: redactService(service) };
}
