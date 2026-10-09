/** API group and version of the Koptan CRDs (see koptan/api/v1). */
export const KOPTAN_GROUP = 'koptan.felukka.org';
export const KOPTAN_VERSION = 'v1';

/** Plural resource names for each Koptan kind. */
export const KOPTAN_PLURALS = {
  Service: 'services',
  CI: 'cis',
  CD: 'cds',
  CIPlugin: 'ciplugins',
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

/** How the image is built (Service spec.build). */
export interface BuildSpec {
  /** Build context relative to the repository root (monorepos). */
  contextDir?: string;
  /** The repository's Dockerfile, relative to the root. */
  dockerfilePath?: string;
  /** Skips detection and generates the Dockerfile for this stack. */
  language?: string;
}

/** What discovery learned about the repository. */
export interface DetectedStack {
  language?: string;
  version?: string;
  packageManager?: string;
  framework?: string;
  entrypoint?: string;
}

/** What the user declares: a git repo. The operator derives CI and CD. */
export interface Service {
  metadata: ObjectMeta;
  spec: {
    source: SourceRef;
    env?: EnvVar[];
    image?: { registry?: string; repo?: string; credentialsSecret?: string };
    replicas?: number;
    port?: number;
    build?: BuildSpec;
    /** CIPlugins this Service runs, by name. */
    plugins?: { name: string }[];
  };
  status?: {
    phase?: ServicePhase;
    /** Detected language, or "dockerfile" for a repository Dockerfile only. */
    serviceType?: string;
    /** Where the Dockerfile came from. */
    dockerfileSource?: 'repo' | 'template';
    detected?: DetectedStack;
    latestRevision?: string;
    lastPushDetected?: string;
    ciRef?: string;
    cdRef?: string;
    dockerfileConfigMap?: string;
    observedGeneration?: number;
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
    image: { registry: string; repo: string; credentialsSecret?: string };
    /** Commit SHA to build. */
    revision?: string;
    contextDir?: string;
    /** Resolved plugins, in run order. */
    plugins?: { name: string; generation?: number }[];
  };
  status?: {
    phase?: CIPhase;
    latestRevision?: string;
    latestImage?: string;
    buildCount?: number;
    lastBuildTime?: string;
    buildPod?: string;
    buildingRevision?: string;
    pluginResults?: PluginResult[];
    message?: string;
    conditions?: Condition[];
  };
}

export type PluginPhase = 'Pending' | 'Running' | 'Succeeded' | 'Failed';

/** One plugin step of a CI's latest build. */
export interface PluginResult {
  name: string;
  phase: PluginPhase;
  message?: string;
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
    port?: number;
    imagePullSecret?: string;
  };
  status?: {
    phase?: CDPhase;
    latestRevision?: string;
    latestImage?: string;
    availableReplicas?: number;
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
  /** Where the image goes; empty fields use the operator defaults. */
  image?: {
    registry?: string;
    repo?: string;
    /** Write-only: stored as dockerconfigjson Secret `<name>-registry`. */
    username?: string;
    password?: string;
  };
  replicas?: number;
  port?: number;
  /** CIPlugins to run after checkout, by name. */
  plugins?: string[];
  /** Build context inside the repository, for monorepos. */
  contextDir?: string;
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

export type CIPluginType = 'sonarqube' | 'codeql' | 'custom';

/** A pluggable CI step (SonarQube, CodeQL, custom) run before build/push. */
export interface CIPlugin {
  metadata: ObjectMeta;
  spec: {
    type: CIPluginType;
    order?: number;
    failurePolicy?: 'Fail' | 'Ignore';
    targetRefs?: { kind?: string; name: string }[];
    selector?: { matchLabels?: Record<string, string> };
    sonarqube?: { hostURL: string; projectKey?: string };
    codeql?: {
      languages?: string[];
      querySuite?: string;
      failOnSeverity?: string;
    };
    custom?: { image: string; command?: string[]; args?: string[] };
  };
  status?: {
    attachedServices?: string[];
    conditions?: Condition[];
  };
}

/** The latest plugin results of one Service's CI. */
export interface PluginRun {
  service: string;
  namespace?: string;
  ci: string;
  revision?: string;
  phase?: CIPhase;
  results: PluginResult[];
}

/** GET /plugins: every CIPlugin and the latest runs. */
export interface ScanBay {
  plugins: CIPlugin[];
  runs: PluginRun[];
}
