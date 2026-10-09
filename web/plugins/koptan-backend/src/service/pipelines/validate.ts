import type { CreatePipelineRequest } from '@internal/plugin-koptan-common';

export const DNS_LABEL = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;
const SCP_URL = /^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._/~-]+$/;
const REVISION = /^[A-Za-z0-9._/][A-Za-z0-9._/-]*$/;
const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
const REGISTRY = /^[A-Za-z0-9.-]+(:[0-9]+)?(\/[A-Za-z0-9._-]+)*$/;
const IMAGE_REPO =
  /^[a-z0-9]+([._-][a-z0-9]+)*(\/[a-z0-9]+([._-][a-z0-9]+)*)*$/;

/**
 * Only remote git URLs: https, http, ssh, git, or user@host:path. Rejects
 * local paths, file://, ext:: and anything git could read as an option.
 * Mirrors ValidateGitURL in the operator (internal/utils/url.go).
 */
export function isValidRepoUrl(repo: string): boolean {
  if (!repo || repo.startsWith('-') || /\s/.test(repo)) return false;
  if (SCP_URL.test(repo)) return true;
  try {
    const u = new URL(repo);
    return (
      ['https:', 'http:', 'ssh:', 'git:'].includes(u.protocol) &&
      !!u.hostname &&
      u.pathname.length > 1
    );
  } catch {
    return false;
  }
}

/** Checks the fields every resource request shares: name and namespace. */
export function validateName(req: {
  name?: string;
  namespace?: string;
}): string | undefined {
  if (!DNS_LABEL.test(req.name ?? '')) return 'name must be a DNS-1123 label';
  if (req.namespace !== undefined && !DNS_LABEL.test(req.namespace))
    return 'namespace must be a DNS-1123 label';
  return undefined;
}

export function validateRevision(revision?: string): string | undefined {
  if (
    revision &&
    (revision.length > 250 ||
      !REVISION.test(revision) ||
      revision.includes('..'))
  )
    return 'revision must be a branch, tag or commit SHA';
  return undefined;
}

export function validateRuntime(req: {
  env?: { name?: string }[];
  replicas?: number;
  port?: number;
}): string | undefined {
  for (const e of req.env ?? []) {
    if (!ENV_NAME.test(e.name ?? ''))
      return `env variable name "${e.name ?? ''}" is not valid`;
  }
  if (
    req.replicas !== undefined &&
    (!Number.isInteger(req.replicas) || req.replicas < 0 || req.replicas > 50)
  )
    return 'replicas must be a whole number from 0 to 50';
  if (
    req.port !== undefined &&
    (!Number.isInteger(req.port) || req.port < 1 || req.port > 65535)
  )
    return 'port must be 1-65535';
  return undefined;
}

export function validateImage(
  img: CreatePipelineRequest['image'],
): string | undefined {
  if (!img) return undefined;
  if (img.registry && !REGISTRY.test(img.registry))
    return 'image.registry must be a registry host, e.g. ghcr.io';
  if (img.repo && !IMAGE_REPO.test(img.repo))
    return 'image.repo must be a lowercase image path, e.g. team/app';
  if (!!img.username !== !!img.password)
    return 'registry username and password must be set together';
  if (img.username && !img.registry)
    return 'set image.registry when giving registry credentials';
  return undefined;
}

export function validateCreate(req: CreatePipelineRequest): string | undefined {
  const named = validateName(req);
  if (named) return named;
  if (!req.repo) return 'repo is required';
  if (!isValidRepoUrl(req.repo))
    return 'repo must be an https://, http://, ssh:// or git:// URL, or user@host:path';
  return (
    validateRevision(req.revision) ??
    validateRuntime(req) ??
    validateImage(req.image)
  );
}
