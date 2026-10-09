import { Readable } from 'node:stream';
import type {
  KoptanClient,
  KoptanKind,
  ProxyRequest,
  ProxyTarget,
  RawResource,
} from '../k8s';

/** Everything a FakeClient was asked to do, for assertions. */
export interface FakeCalls {
  created: [KoptanKind, RawResource][];
  secrets: [string, Record<string, string>, string | undefined][];
  owners: [string, string][];
  deleted: string[];
  proxied: [ProxyTarget, ProxyRequest][];
  annotated: [KoptanKind, string, Record<string, string>][];
}

export interface FakeOptions {
  /** Resources returned by list/get, by kind. */
  objects?: Partial<Record<KoptanKind, RawResource[]>>;
  /** Thrown by create, e.g. { code: 409 }. */
  failCreate?: unknown;
  /** Secret values by `<name>/<key>`. */
  secretValues?: Record<string, string>;
  /** Body and status the proxy answers with. */
  proxyReply?: { status: number; body: string; contentType?: string };
}

/** An in-memory KoptanClient that records its calls. */
export function fakeClient(opts: FakeOptions = {}) {
  const calls: FakeCalls = {
    created: [],
    secrets: [],
    owners: [],
    deleted: [],
    proxied: [],
    annotated: [],
  };
  const objects = (kind: KoptanKind) => opts.objects?.[kind] ?? [];
  const client: KoptanClient = {
    list: async (kind) => objects(kind),
    get: async (kind, ns, name) => {
      const found = objects(kind).find(
        (o) =>
          o.metadata?.name === name && (o.metadata?.namespace ?? ns) === ns,
      );
      if (!found) throw { code: 404, body: { message: `${name} not found` } };
      return found;
    },
    create: async (kind, _ns, body) => {
      if (opts.failCreate) throw opts.failCreate;
      calls.created.push([kind, body]);
      return { ...body, metadata: { ...body.metadata, uid: 'uid-1' } };
    },
    annotate: async (kind, _ns, name, annotations) => {
      calls.annotated.push([kind, name, annotations]);
    },
    createSecret: async (_ns, name, data, type) => {
      calls.secrets.push([name, data, type]);
    },
    readSecretKey: async (_ns, name, key) =>
      opts.secretValues?.[`${name}/${key}`],
    setSecretOwner: async (_ns, name, owner) => {
      calls.owners.push([name, owner.uid]);
    },
    deleteSecret: async (_ns, name) => {
      calls.deleted.push(name);
    },
    clusterInfo: async () => ({ nodes: { total: 0, ready: 0 } }),
    proxy: async (target, init) => {
      calls.proxied.push([target, init]);
      const reply = opts.proxyReply ?? { status: 200, body: '' };
      return {
        status: reply.status,
        contentType: reply.contentType,
        body: Readable.from([reply.body]),
      };
    },
  };
  return { client, calls };
}
