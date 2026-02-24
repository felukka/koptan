import {
  ConflictError,
  InputError,
  NotAllowedError,
  NotFoundError,
  ServiceUnavailableError,
} from '@backstage/errors';

type ApiError = { code?: number; body?: unknown };

const CRDS_MISSING =
  'The Koptan CRDs are not installed in this cluster; run `make install` in the koptan repository';

/** The `message` of a Kubernetes API error body, if there is one. */
function apiMessage(err: ApiError): string | undefined {
  try {
    const body = typeof err.body === 'string' ? JSON.parse(err.body) : err.body;
    return (body as { message?: string })?.message;
  } catch {
    return undefined;
  }
}

/** A 404 with no Status body comes from a missing resource type, not object. */
function isMissingType(err: ApiError): boolean {
  if (err.code !== 404) return false;
  const msg = apiMessage(err);
  return !msg || /could not find the requested resource/.test(msg);
}

/** Turns Kubernetes API failures on a read into short, readable errors. */
export function friendlyReadError(e: unknown): unknown {
  const err = e as ApiError;
  if (isMissingType(err)) return new ServiceUnavailableError(CRDS_MISSING);
  if (err.code === 404)
    return new NotFoundError(apiMessage(err) ?? 'Not found');
  if (err.code === 403) {
    return new NotAllowedError(
      'The service account used by Backstage may not read these resources; see config/rbac/backstage_role.yaml',
    );
  }
  return e;
}

/** Turns Kubernetes API failures on a create into short, readable errors. */
export function friendlyError(
  e: unknown,
  target: { name: string; namespace?: string },
): unknown {
  const err = e as ApiError;
  const ns = target.namespace ?? 'default';
  if (err.code === 409) {
    return new ConflictError(
      `"${target.name}" already exists in namespace "${ns}" (a resource or secret with that name)`,
    );
  }
  if (err.code === 403) {
    return new NotAllowedError(
      'The service account used by Backstage is not allowed to create these resources; see config/rbac/backstage_role.yaml',
    );
  }
  if (err.code === 404) {
    const msg = apiMessage(err) ?? '';
    if (/namespaces? .* not found/.test(msg)) {
      return new InputError(`Namespace "${ns}" does not exist`);
    }
    return new InputError(
      isMissingType(err) ? CRDS_MISSING : `Not found: ${msg}`,
    );
  }
  if (err.code === 422 || err.code === 400) {
    return new InputError(
      apiMessage(err) ?? 'The cluster rejected the request',
    );
  }
  return e;
}
