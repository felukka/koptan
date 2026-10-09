export { friendlyError, friendlyReadError } from './errors';
export { KubeKoptanClient } from './kubeClient';
export { createWithSecrets, type SecretSpec } from './ownedSecrets';
export type {
  KoptanClient,
  KoptanKind,
  OwnerRef,
  ProxyRequest,
  ProxyResponse,
  ProxyTarget,
  RawResource,
} from './types';
