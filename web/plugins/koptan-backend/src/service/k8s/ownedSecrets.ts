import { KOPTAN_GROUP, KOPTAN_VERSION } from '@internal/plugin-koptan-common';
import type { KoptanClient, KoptanKind, RawResource } from './types';

export interface SecretSpec {
  name: string;
  data: Record<string, string>;
  type?: string;
}

/**
 * Creates the Secrets, then the resource, then makes the resource own the
 * Secrets so they are deleted with it. Secrets come first so the operator
 * can read them; if the resource cannot be created they are deleted again.
 */
export async function createWithSecrets(
  client: KoptanClient,
  kind: KoptanKind,
  namespace: string,
  secrets: SecretSpec[],
  body: RawResource,
): Promise<RawResource> {
  const created: string[] = [];
  let resource: RawResource;
  try {
    for (const s of secrets) {
      await client.createSecret(namespace, s.name, s.data, s.type);
      created.push(s.name);
    }
    resource = await client.create(kind, namespace, body);
  } catch (e) {
    for (const name of created) {
      await client.deleteSecret(namespace, name).catch(() => undefined);
    }
    throw e;
  }

  const uid = resource.metadata?.uid;
  if (uid) {
    const owner = {
      apiVersion: `${KOPTAN_GROUP}/${KOPTAN_VERSION}`,
      kind,
      name: body.metadata.name,
      uid,
    };
    await Promise.all(
      created.map((name) => client.setSecretOwner(namespace, name, owner)),
    );
  }
  return { kind, ...resource };
}
