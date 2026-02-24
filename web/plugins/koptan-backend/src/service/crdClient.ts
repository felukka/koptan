import {
  CoreV1Api,
  CustomObjectsApi,
  KubeConfig,
  PatchStrategy,
  setHeaderOptions,
  VersionApi,
} from '@kubernetes/client-node';
import {
  type ClusterInfo,
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
  /** Creates a Secret (Opaque unless a type is given) holding string data. */
  createSecret(
    namespace: string,
    name: string,
    data: Record<string, string>,
    type?: string,
  ): Promise<void>;
  /** Makes a Secret owned by a resource, so it is deleted with it. */
  setSecretOwner(
    namespace: string,
    name: string,
    owner: OwnerRef,
  ): Promise<void>;
  deleteSecret(namespace: string, name: string): Promise<void>;
  clusterInfo(): Promise<ClusterInfo>;
}

export interface OwnerRef {
  apiVersion: string;
  kind: string;
  name: string;
  uid: string;
}

export class KubeKoptanClient implements KoptanClient {
  private cached?: CustomObjectsApi;
  private loaded?: KubeConfig;

  /** Takes a loader so a missing or broken kubeconfig also fails per request. */
  constructor(private readonly source: KubeConfig | (() => KubeConfig)) {}

  private get kubeConfig(): KubeConfig {
    this.loaded ??=
      typeof this.source === 'function' ? this.source() : this.source;
    return this.loaded;
  }

  /**
   * Built on first use, so the backend still starts without a usable cluster
   * (for example no current kube context); requests then fail with the reason.
   */
  private get api(): CustomObjectsApi {
    this.cached ??= this.kubeConfig.makeApiClient(CustomObjectsApi);
    return this.cached;
  }

  static fromDefault(context?: string): KubeKoptanClient {
    return new KubeKoptanClient(() => {
      const kc = new KubeConfig();
      kc.loadFromDefault();
      if (context) {
        kc.setCurrentContext(context);
      }
      return kc;
    });
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

  async createSecret(
    namespace: string,
    name: string,
    data: Record<string, string>,
    type = 'Opaque',
  ): Promise<void> {
    await this.kubeConfig.makeApiClient(CoreV1Api).createNamespacedSecret({
      namespace,
      body: { metadata: { name, namespace }, type, stringData: data },
    });
  }

  async setSecretOwner(
    namespace: string,
    name: string,
    owner: OwnerRef,
  ): Promise<void> {
    await this.kubeConfig.makeApiClient(CoreV1Api).patchNamespacedSecret(
      {
        namespace,
        name,
        body: { metadata: { ownerReferences: [owner] } },
      },
      setHeaderOptions('Content-Type', PatchStrategy.MergePatch),
    );
  }

  async deleteSecret(namespace: string, name: string): Promise<void> {
    await this.kubeConfig
      .makeApiClient(CoreV1Api)
      .deleteNamespacedSecret({ namespace, name });
  }

  async clusterInfo(): Promise<ClusterInfo> {
    const info: ClusterInfo = { nodes: { total: 0, ready: 0 } };
    const errors: string[] = [];
    try {
      const v = await this.kubeConfig.makeApiClient(VersionApi).getCode();
      info.kubernetesVersion = v.gitVersion;
    } catch (e) {
      errors.push((e as Error).message);
    }
    try {
      const nodes = await this.kubeConfig.makeApiClient(CoreV1Api).listNode();
      info.nodes.total = nodes.items.length;
      info.nodes.ready = nodes.items.filter((n) =>
        n.status?.conditions?.some(
          (c) => c.type === 'Ready' && c.status === 'True',
        ),
      ).length;
    } catch (e) {
      errors.push((e as Error).message);
    }
    if (errors.length) info.error = errors.join('; ');
    return info;
  }
}
