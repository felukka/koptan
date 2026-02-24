/** API group and version of the Koptan CRDs (see koptan/api/v1). */
export const KOPTAN_GROUP = 'koptan.felukka.org';
export const KOPTAN_VERSION = 'v1';

/** Plural resource names for each Koptan kind. */
export const KOPTAN_PLURALS = {
  Service: 'services',
  CI: 'cis',
  CD: 'cds',
  CIPlugin: 'ciplugins',
  Alert: 'alerts',
  SelfService: 'selfservices',
} as const;

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

export interface EnvVar {
  name: string;
  value?: string;
}
