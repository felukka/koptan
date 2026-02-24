import type { Condition, ObjectMeta } from './meta';

export type AlertEvent =
  | 'Push'
  | 'CIStarted'
  | 'CISucceeded'
  | 'CIFailed'
  | 'CDDeploying'
  | 'CDSucceeded'
  | 'CDFailed';

export const ALERT_EVENTS: AlertEvent[] = [
  'Push',
  'CIStarted',
  'CISucceeded',
  'CIFailed',
  'CDDeploying',
  'CDSucceeded',
  'CDFailed',
];

export type ChannelType = 'slack' | 'teams' | 'webhook';

/** One notification attempt. */
export interface AlertDelivery {
  time: string;
  service: string;
  event: AlertEvent;
  channel: string;
  revision?: string;
  success: boolean;
  error?: string;
}

/** Sends a Service's pipeline events to Slack, Teams or a webhook. */
export interface Alert {
  metadata: ObjectMeta;
  spec: {
    serviceRef?: { name: string };
    selector?: { matchLabels?: Record<string, string> };
    events?: AlertEvent[];
    /** Channel URLs live in Secrets and are never returned. */
    channels: {
      name: string;
      type: ChannelType;
      urlSecretRef: { name: string; key: string };
      signingSecretRef?: { name: string; key: string };
    }[];
    suspend?: boolean;
  };
  status?: {
    deliveries?: AlertDelivery[];
    conditions?: Condition[];
  };
}

/** An AlertDelivery with the Alert it belongs to, for the feed. */
export interface DeliveryEntry extends AlertDelivery {
  alert: string;
  namespace?: string;
}

/** GET /alerts: every Alert and their recent deliveries, newest first. */
export interface SignalMast {
  alerts: Alert[];
  deliveries: DeliveryEntry[];
}

/** Body of POST /alerts. */
export interface CreateAlertRequest {
  name: string;
  namespace?: string;
  /** One Service by name, or every Service with these labels. */
  service?: string;
  selector?: Record<string, string>;
  /** Empty means every event. */
  events?: AlertEvent[];
  channels: {
    name: string;
    type: ChannelType;
    /** Write-only: stored in Secret `<alert>-channels`. */
    url: string;
    /** Write-only: webhook HMAC key, stored in the same Secret. */
    signingKey?: string;
  }[];
}
