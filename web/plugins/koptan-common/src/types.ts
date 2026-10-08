/** API group and version of the Koptan CRDs (see koptan/api/v1). */
export const KOPTAN_GROUP = 'koptan.felukka.org';
export const KOPTAN_VERSION = 'v1';

/** Plural resource names for each Koptan kind. */
export const KOPTAN_PLURALS = {
  Service: 'services',
  CI: 'cis',
  CD: 'cds',
} as const;

export type ServicePhase =
  | 'Pending'
  | 'Discovering'
  | 'Building'
  | 'Ready'
  | 'Failed';
export type CIPhase =
  | 'Idle'
  | 'Resolving'
  | 'Building'
  | 'Succeeded'
  | 'Failed';
export type CDPhase = 'Waiting' | 'Deploying' | 'Running' | 'Failed';

export interface Condition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
}

export interface ObjectMeta {
  name: string;
  namespace?: string;
  creationTimestamp?: string;
  generation?: number;
}

export interface SourceRef {
  repo: string;
  revision?: string;
  /** Secret holding the git token; the token itself is never returned. */
  secretRef?: { name: string; key: string };
}

export interface EnvVar {
  name: string;
  value?: string;
}

/** What the user declares: a git repo. The operator derives CI and CD. */
export interface Service {
  metadata: ObjectMeta;
  spec: {
    source: SourceRef;
    env?: EnvVar[];
  };
  status?: {
    phase?: ServicePhase;
    /** Detected language: go, java or dotnet. */
    serviceType?: string;
    latestRevision?: string;
    lastPushDetected?: string;
    ciRef?: string;
    cdRef?: string;
    error?: string;
    message?: string;
    conditions?: Condition[];
  };
}

/** Image build for a Service. */
export interface CI {
  metadata: ObjectMeta;
  spec: {
    service: { name: string };
    /** Registry login is stripped by the backend. */
    image: { registry: string; repo: string };
  };
  status?: {
    phase?: CIPhase;
    latestRevision?: string;
    latestImage?: string;
    buildCount?: number;
    lastBuildTime?: string;
    message?: string;
    conditions?: Condition[];
  };
}

/** Deployment of a CI's image. */
export interface CD {
  metadata: ObjectMeta;
  spec: {
    ci: { name: string };
    replicas?: number;
    env?: EnvVar[];
    resources?: {
      cpuRequest?: string;
      cpuLimit?: string;
      memoryRequest?: string;
      memoryLimit?: string;
    };
  };
  status?: {
    phase?: CDPhase;
    latestRevision?: string;
    latestImage?: string;
    message?: string;
    conditions?: Condition[];
  };
}

/** One Service -> CI -> CD chain, joined by the backend. */
export interface Pipeline {
  service: Service;
  ci?: CI;
  cd?: CD;
}

export interface Overview {
  services: { total: number; byPhase: Record<string, number> };
  cis: { total: number; byPhase: Record<string, number> };
  cds: { total: number; byPhase: Record<string, number> };
  replicas: { desired: number };
}

/** Body of POST /pipelines: creates a Service; the operator creates CI and CD. */
export interface CreatePipelineRequest {
  name: string;
  namespace?: string;
  repo: string;
  revision?: string;
  /** Write-only: stored as Secret `<name>-git`, never returned. */
  token?: string;
  env?: EnvVar[];
}

/** Cluster facts shown in the Bay metrics bar. */
export interface ClusterInfo {
  kubernetesVersion?: string;
  nodes: { total: number; ready: number };
  /** Set when the cluster could not be queried for these facts. */
  error?: string;
}

/** One line of the Bay activity log, newest first. */
export interface ActivityEntry {
  time: string;
  kind: 'Service' | 'CI' | 'CD';
  name: string;
  namespace?: string;
  message: string;
  severity: 'info' | 'success' | 'error';
}
