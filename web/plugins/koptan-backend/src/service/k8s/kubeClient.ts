import http from 'node:http';
import https from 'node:https';
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
import type {
  KoptanClient,
  KoptanKind,
  OwnerRef,
  ProxyRequest,
  ProxyResponse,
  ProxyTarget,
  RawResource,
} from './types';

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

  private get core(): CoreV1Api {
    return this.kubeConfig.makeApiClient(CoreV1Api);
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

  private base(kind: KoptanKind) {
    return {
      group: KOPTAN_GROUP,
      version: KOPTAN_VERSION,
      plural: KOPTAN_PLURALS[kind],
    };
  }

  async list(kind: KoptanKind, namespace?: string): Promise<RawResource[]> {
    const base = this.base(kind);
    const res = (
      namespace
        ? await this.api.listNamespacedCustomObject({ ...base, namespace })
        : await this.api.listClusterCustomObject(base)
    ) as { items?: RawResource[] };
    // The CRDs omit `kind` on list items, so stamp it for the join logic.
    return (res.items ?? []).map((item) => ({ kind, ...item }));
  }

  async get(
    kind: KoptanKind,
    namespace: string,
    name: string,
  ): Promise<RawResource> {
    const res = (await this.api.getNamespacedCustomObject({
      ...this.base(kind),
      namespace,
      name,
    })) as RawResource;
    return { kind, ...res };
  }

  async create(
    kind: KoptanKind,
    namespace: string,
    body: RawResource,
  ): Promise<RawResource> {
    return (await this.api.createNamespacedCustomObject({
      ...this.base(kind),
      namespace,
      body: {
        apiVersion: `${KOPTAN_GROUP}/${KOPTAN_VERSION}`,
        kind,
        ...body,
      },
    })) as RawResource;
  }

  async annotate(
    kind: KoptanKind,
    namespace: string,
    name: string,
    annotations: Record<string, string>,
  ): Promise<void> {
    await this.api.patchNamespacedCustomObject(
      {
        ...this.base(kind),
        namespace,
        name,
        body: { metadata: { annotations } },
      },
      setHeaderOptions('Content-Type', PatchStrategy.MergePatch),
    );
  }

  async createSecret(
    namespace: string,
    name: string,
    data: Record<string, string>,
    type = 'Opaque',
  ): Promise<void> {
    await this.core.createNamespacedSecret({
      namespace,
      body: { metadata: { name, namespace }, type, stringData: data },
    });
  }

  async readSecretKey(
    namespace: string,
    name: string,
    key: string,
  ): Promise<string | undefined> {
    const secret = await this.core.readNamespacedSecret({ namespace, name });
    const value = secret.data?.[key];
    return value === undefined
      ? undefined
      : Buffer.from(value, 'base64').toString('utf8');
  }

  async setSecretOwner(
    namespace: string,
    name: string,
    owner: OwnerRef,
  ): Promise<void> {
    await this.core.patchNamespacedSecret(
      {
        namespace,
        name,
        body: { metadata: { ownerReferences: [owner] } },
      },
      setHeaderOptions('Content-Type', PatchStrategy.MergePatch),
    );
  }

  async deleteSecret(namespace: string, name: string): Promise<void> {
    await this.core.deleteNamespacedSecret({ namespace, name });
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
      const nodes = await this.core.listNode();
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

  async proxy(target: ProxyTarget, init: ProxyRequest): Promise<ProxyResponse> {
    const cluster = this.kubeConfig.getCurrentCluster();
    if (!cluster) throw new Error('no current Kubernetes cluster');
    const path =
      `/api/v1/namespaces/${encodeURIComponent(target.namespace)}` +
      `/services/${encodeURIComponent(`${target.service}:${target.port}`)}` +
      `/proxy${target.path}`;
    const url = new URL(path, cluster.server);
    const opts: https.RequestOptions = {
      method: init.method,
      headers: { ...init.headers },
      signal: init.signal,
    };
    await this.kubeConfig.applyToHTTPSOptions(opts);
    const transport = url.protocol === 'https:' ? https : http;
    return new Promise((resolve, reject) => {
      const req = transport.request(url, opts, (res) =>
        resolve({
          status: res.statusCode ?? 502,
          contentType: res.headers['content-type'],
          body: res,
        }),
      );
      req.on('error', reject);
      if (init.body) req.write(init.body);
      req.end();
    });
  }
}
