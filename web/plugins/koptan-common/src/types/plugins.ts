import type { CIPhase, PluginResult } from './pipeline';
import type { Condition, ObjectMeta } from './meta';

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
