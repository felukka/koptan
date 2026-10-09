import type { CD, CI, Service } from '@internal/plugin-koptan-common';
import type { RawResource } from '../k8s';

const LAST_APPLIED = 'kubectl.kubernetes.io/last-applied-configuration';

/**
 * `kubectl apply` stores a full copy of the spec (tokens included) in the
 * last-applied annotation, and managedFields is noise, so drop both.
 */
export function cleanMetadata(raw: RawResource): RawResource {
  const { managedFields: _mf, annotations, ...meta } = raw.metadata ?? {};
  const { [LAST_APPLIED]: _la, ...rest } = annotations ?? {};
  return Object.keys(rest).length ? { ...meta, annotations: rest } : meta;
}

/** Keeps a resource as-is apart from metadata noise. */
export function clean<T>(raw: RawResource): T {
  return { ...raw, metadata: cleanMetadata(raw) } as T;
}

/** The secretRef only names a Secret, so nothing to strip beyond noise. */
export function redactService(raw: RawResource): Service {
  return clean<Service>(raw);
}

/** Drops the registry login so credentials never reach the browser. */
export function redactCI(raw: RawResource): CI {
  const { loginSecret: _omit, ...image } = raw.spec?.image ?? {};
  return {
    ...raw,
    metadata: cleanMetadata(raw),
    spec: { ...raw.spec, image },
  } as CI;
}

export function redactCD(raw: RawResource): CD {
  return clean<CD>(raw);
}
