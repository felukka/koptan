import type {
  BackstageCredentials,
  BackstageUserPrincipal,
  HttpAuthService,
  PermissionsService,
} from '@backstage/backend-plugin-api';
import { NotAllowedError } from '@backstage/errors';
import {
  AuthorizeResult,
  type BasicPermission,
} from '@backstage/plugin-permission-common';
import type { Request } from 'express';
import { friendlyReadError, type KoptanClient } from '../k8s';

/** What every route module needs. */
export interface RouteContext {
  client: KoptanClient;
  httpAuth: HttpAuthService;
  permissions: PermissionsService;
  /** Limits reads to one namespace; unset means all namespaces. */
  namespace?: string;
}

/**
 * Resolves the calling user and checks one permission; throws
 * NotAllowedError (403) when it is denied.
 */
export async function authorize(
  ctx: RouteContext,
  req: Request,
  permission: BasicPermission,
): Promise<BackstageCredentials<BackstageUserPrincipal>> {
  const credentials = await ctx.httpAuth.credentials(req, { allow: ['user'] });
  const [decision] = await ctx.permissions.authorize([{ permission }], {
    credentials,
  });
  if (decision.result !== AuthorizeResult.ALLOW) {
    throw new NotAllowedError('Not allowed');
  }
  return credentials;
}

/** Runs a read against the cluster, translating Kubernetes errors. */
export async function read<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (e) {
    throw friendlyReadError(e);
  }
}
