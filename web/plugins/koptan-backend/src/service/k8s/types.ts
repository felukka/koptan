import type { Readable } from 'node:stream';
import type {
  ClusterInfo,
  KOPTAN_PLURALS,
} from '@internal/plugin-koptan-common';

export type KoptanKind = keyof typeof KOPTAN_PLURALS;

/** Raw custom resource as returned by the Kubernetes API. */
// biome-ignore lint/suspicious/noExplicitAny: untyped Kubernetes JSON
export type RawResource = Record<string, any>;

export interface OwnerRef {
  apiVersion: string;
  kind: string;
  name: string;
  uid: string;
}

/** Minimal client over the Koptan CRDs and the Secrets they use; faked in tests. */
export interface KoptanClient {
  list(kind: KoptanKind, namespace?: string): Promise<RawResource[]>;
  get(kind: KoptanKind, namespace: string, name: string): Promise<RawResource>;
  create(
    kind: KoptanKind,
    namespace: string,
    body: RawResource,
  ): Promise<RawResource>;
  /** Merges annotations into a Koptan resource. */
  annotate(
    kind: KoptanKind,
    namespace: string,
    name: string,
    annotations: Record<string, string>,
  ): Promise<void>;
  /** Creates a Secret (Opaque unless a type is given) holding string data. */
  createSecret(
    namespace: string,
    name: string,
    data: Record<string, string>,
    type?: string,
  ): Promise<void>;
  /** Reads one key of a Secret, decoded; undefined when the key is absent. */
  readSecretKey(
    namespace: string,
    name: string,
    key: string,
  ): Promise<string | undefined>;
  /** Makes a Secret owned by a resource, so it is deleted with it. */
  setSecretOwner(
    namespace: string,
    name: string,
    owner: OwnerRef,
  ): Promise<void>;
  deleteSecret(namespace: string, name: string): Promise<void>;
  clusterInfo(): Promise<ClusterInfo>;
  /**
   * Sends a request to a Service in the cluster through the API server's
   * service proxy, so the backend need not run inside the cluster.
   */
  proxy(target: ProxyTarget, init: ProxyRequest): Promise<ProxyResponse>;
}

export interface ProxyTarget {
  namespace: string;
  service: string;
  port: number;
  path: string;
}

export interface ProxyRequest {
  method: 'GET' | 'POST';
  headers?: Record<string, string>;
  body?: string;
  signal?: AbortSignal;
}

export interface ProxyResponse {
  status: number;
  contentType?: string;
  /** The response body, streamed (server-sent events pass straight through). */
  body: Readable;
}
