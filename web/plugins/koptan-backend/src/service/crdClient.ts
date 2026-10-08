import { CustomObjectsApi, KubeConfig } from '@kubernetes/client-node';
import {
  KOPTAN_GROUP,
  KOPTAN_PLURALS,
  KOPTAN_VERSION,
} from '@internal/plugin-koptan-common';

export type KoptanKind = keyof typeof KOPTAN_PLURALS;

/** Raw custom resource as returned by the Kubernetes API. */
// biome-ignore lint/suspicious/noExplicitAny: untyped Kubernetes JSON
export type RawResource = Record<string, any>;

/** Minimal client over the Koptan CRDs; faked in tests. */
export interface KoptanClient {
  list(kind: KoptanKind, namespace?: string): Promise<RawResource[]>;
  create(
    kind: KoptanKind,
    namespace: string,
    body: RawResource,
  ): Promise<RawResource>;
}

export class KubeKoptanClient implements KoptanClient {
  private readonly api: CustomObjectsApi;

  constructor(kubeConfig: KubeConfig) {
    this.api = kubeConfig.makeApiClient(CustomObjectsApi);
  }

  static fromDefault(context?: string): KubeKoptanClient {
    const kc = new KubeConfig();
    kc.loadFromDefault();
    if (context) {
      kc.setCurrentContext(context);
    }
    return new KubeKoptanClient(kc);
  }

  async list(kind: KoptanKind, namespace?: string): Promise<RawResource[]> {
    const base = {
      group: KOPTAN_GROUP,
      version: KOPTAN_VERSION,
      plural: KOPTAN_PLURALS[kind],
    };
    const res = (
      namespace
        ? await this.api.listNamespacedCustomObject({ ...base, namespace })
        : await this.api.listClusterCustomObject(base)
    ) as { items?: RawResource[] };
    // The CRDs omit `kind` on list items, so stamp it for the join logic.
    return (res.items ?? []).map((item) => ({ kind, ...item }));
  }

  async create(
    kind: KoptanKind,
    namespace: string,
    body: RawResource,
  ): Promise<RawResource> {
    return (await this.api.createNamespacedCustomObject({
      group: KOPTAN_GROUP,
      version: KOPTAN_VERSION,
      namespace,
      plural: KOPTAN_PLURALS[kind],
      body: {
        apiVersion: `${KOPTAN_GROUP}/${KOPTAN_VERSION}`,
        kind,
        ...body,
      },
    })) as RawResource;
  }
}
