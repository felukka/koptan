import { fetchApiRef, useApi } from '@backstage/frontend-plugin-api';
import type {
  ActivityEntry,
  Alert,
  ClusterInfo,
  CreateAlertRequest,
  CreatePipelineRequest,
  Overview,
  Pipeline,
  ScanBay,
  SignalMast,
} from '@internal/plugin-koptan-common';
import { useCallback } from 'react';

const post = (body: unknown): RequestInit => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      message = body?.error?.message ?? message;
    } catch {
      // keep the status text
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

/** Calls the koptan backend plugin through the Backstage fetch API. */
export function useKoptanApi() {
  const { fetch } = useApi(fetchApiRef);
  const getOverview = useCallback(
    async () => json<Overview>(await fetch('plugin://koptan/overview')),
    [fetch],
  );
  const getPipelines = useCallback(
    async () => json<Pipeline[]>(await fetch('plugin://koptan/pipelines')),
    [fetch],
  );
  const getActivity = useCallback(
    async () => json<ActivityEntry[]>(await fetch('plugin://koptan/activity')),
    [fetch],
  );
  const getCluster = useCallback(
    async () => json<ClusterInfo>(await fetch('plugin://koptan/cluster')),
    [fetch],
  );
  const getScanBay = useCallback(
    async () => json<ScanBay>(await fetch('plugin://koptan/plugins')),
    [fetch],
  );
  const createPipeline = useCallback(
    async (req: CreatePipelineRequest) =>
      json<Pipeline>(await fetch('plugin://koptan/pipelines', post(req))),
    [fetch],
  );
  const getSignalMast = useCallback(
    async () => json<SignalMast>(await fetch('plugin://koptan/alerts')),
    [fetch],
  );
  const createAlert = useCallback(
    async (req: CreateAlertRequest) =>
      json<Alert>(await fetch('plugin://koptan/alerts', post(req))),
    [fetch],
  );
  return {
    getOverview,
    getPipelines,
    getActivity,
    getCluster,
    getScanBay,
    createPipeline,
    getSignalMast,
    createAlert,
  };
}
