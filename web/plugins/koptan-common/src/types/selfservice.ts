import type { Condition, EnvVar, ObjectMeta } from './meta';

export type SelfServicePhase = 'Provisioning' | 'Ready' | 'Failed';
export type GitProvider = 'github' | 'gitlab';
export type AIProvider = 'anthropic' | 'openai-compatible';

/** A git repository paired with an agent session that turns prompts into commits. */
export interface SelfService {
  metadata: ObjectMeta;
  spec: {
    repo: {
      existing?: { url: string; secretRef: { name: string; key: string } };
      create?: {
        provider: GitProvider;
        owner?: string;
        name?: string;
        private?: boolean;
        baseURL?: string;
        tokenSecretRef: { name: string; key: string };
      };
    };
    branch?: string;
    ai: {
      provider: AIProvider;
      model: string;
      baseURL?: string;
      apiKeySecretRef?: { name: string; key: string };
    };
    service?: { port?: number; replicas?: number; env?: EnvVar[] };
    allowCommands?: boolean;
    suspend?: boolean;
  };
  status?: {
    phase?: SelfServicePhase;
    repoURL?: string;
    agentService?: string;
    serviceRef?: string;
    message?: string;
    conditions?: Condition[];
  };
}

/** Body of POST /selfservices. Tokens and keys are write-only. */
export interface CreateSelfServiceRequest {
  name: string;
  namespace?: string;
  repo:
    | { mode: 'existing'; url: string; token: string }
    | {
        mode: 'create';
        provider: GitProvider;
        owner?: string;
        repoName?: string;
        private?: boolean;
        baseURL?: string;
        token: string;
      };
  branch?: string;
  ai: {
    provider: AIProvider;
    model: string;
    baseURL?: string;
    apiKey?: string;
  };
  port?: number;
  allowCommands?: boolean;
}

export type AgentRunStatus = 'running' | 'succeeded' | 'failed';

/** One prompt the agent handled. */
export interface AgentRun {
  id: string;
  prompt: string;
  status: AgentRunStatus;
  startedAt: string;
  finishedAt?: string;
  commit?: string | null;
  summary?: string;
  error?: string;
}

/** A progress event streamed while a run works. */
export type AgentEvent =
  | { type: 'status'; text: string }
  | { type: 'message'; text: string }
  | { type: 'tool'; name: string; input: Record<string, unknown> }
  | { type: 'tool_result'; name: string; output: string; isError: boolean }
  | { type: 'done'; commit: string | null; summary: string }
  | { type: 'error'; message: string };
