/** API group and version of the Koptan CRDs (see koptan/api/v1alpha). */
export const KOPTAN_GROUP = 'koptan.felukka.org';
export const KOPTAN_VERSION = 'v1alpha';

export const APP_KINDS = ['GoApp', 'JavaApp', 'DotnetApp'] as const;
export type AppKind = (typeof APP_KINDS)[number];

/** Plural resource names for each Koptan kind. */
export const KOPTAN_PLURALS = {
  GoApp: 'goapps',
  JavaApp: 'javaapps',
  DotnetApp: 'dotnetapps',
  Slipway: 'slipways',
  Voyage: 'voyages',
} as const;

export type AppPhase = 'Pending' | 'Discovering' | 'Ready' | 'Failed';
export type SlipwayPhase =
  | 'Idle'
  | 'Resolving'
  | 'Building'
  | 'Succeeded'
  | 'Failed';
export type VoyagePhase = 'Waiting' | 'Deploying' | 'Running' | 'Failed';

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
  /** Write-only on create; never returned by the backend. */
  patToken?: string;
}

/** Fields shared by GoApp, JavaApp and DotnetApp that the UI cares about. */
export interface KoptanApp {
  kind: AppKind;
  metadata: ObjectMeta;
  spec: {
    source: SourceRef;
    [key: string]: unknown;
  };
  status?: {
    phase?: AppPhase;
    error?: string;
    conditions?: Condition[];
    [key: string]: unknown;
  };
}

export interface Slipway {
  metadata: ObjectMeta;
  spec: {
    appRef: { name: string; kind: AppKind };
    image: {
      registry: string;
      name: string;
      creds?: { username: string; password: string };
    };
  };
  status?: {
    phase?: SlipwayPhase;
    latestRevision?: string;
    latestImage?: string;
    buildCount?: number;
    lastBuildTime?: string;
    message?: string;
    conditions?: Condition[];
  };
}

export interface Voyage {
  metadata: ObjectMeta;
  spec: {
    slipwayRef: { name: string };
    port: number;
    replicas?: number;
    env?: { name: string; value?: string }[];
    healthCheck?: { path?: string; port?: number };
  };
  status?: {
    phase?: VoyagePhase;
    deployedImage?: string;
    conditions?: Condition[];
  };
}

/** One App -> Slipway -> Voyage chain, joined by the backend. */
export interface Pipeline {
  app: KoptanApp;
  slipway?: Slipway;
  voyage?: Voyage;
}

export interface Overview {
  apps: { total: number; byPhase: Record<string, number> };
  slipways: { total: number; byPhase: Record<string, number> };
  voyages: { total: number; byPhase: Record<string, number> };
  replicas: { desired: number };
}

/** Body of POST /pipelines: creates an app, its slipway and its voyage. */
export interface CreatePipelineRequest {
  name: string;
  namespace?: string;
  kind: AppKind;
  source: SourceRef;
  /** Language-specific app spec fields (goVersion, entrypoint, env, ...). */
  appSpec?: Record<string, unknown>;
  slipway: {
    registry: string;
    image: string;
    /** Registry credentials; the CRD stores them in the Slipway spec. */
    username?: string;
    password?: string;
  };
  voyage: {
    port: number;
    replicas?: number;
    /** HTTP path probed for health, e.g. /healthz. */
    healthCheckPath?: string;
  };
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
  kind: 'App' | 'Slipway' | 'Voyage';
  name: string;
  namespace?: string;
  message: string;
  severity: 'info' | 'success' | 'error';
}
