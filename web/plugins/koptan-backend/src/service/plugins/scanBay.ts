import type {
  CI,
  CIPlugin,
  PluginRun,
  ScanBay,
} from '@internal/plugin-koptan-common';
import type { KoptanClient, RawResource } from '../k8s';
import { cleanMetadata, redactCI } from '../pipelines';

/**
 * Keeps what the Scan Bay shows. Custom step env can hold credentials, so
 * env and envFrom are dropped; the other configs only reference Secrets.
 */
export function redactPlugin(raw: RawResource): CIPlugin {
  const { env: _env, envFrom: _envFrom, ...custom } = raw.spec?.custom ?? {};
  return {
    ...raw,
    metadata: cleanMetadata(raw),
    spec: {
      ...raw.spec,
      ...(raw.spec?.custom ? { custom } : {}),
    },
  } as CIPlugin;
}

/** The plugin results of each CI's latest build, newest builds first. */
export function buildRuns(cis: CI[]): PluginRun[] {
  return cis
    .filter((ci) => ci.status?.pluginResults?.length)
    .map((ci) => ({
      service: ci.spec.service.name,
      namespace: ci.metadata.namespace,
      ci: ci.metadata.name,
      revision: ci.status?.buildingRevision ?? ci.spec.revision,
      phase: ci.status?.phase,
      results: ci.status?.pluginResults ?? [],
    }))
    .sort((a, b) => a.service.localeCompare(b.service));
}

export async function loadScanBay(
  client: KoptanClient,
  namespace?: string,
): Promise<ScanBay> {
  const [plugins, cis] = await Promise.all([
    client.list('CIPlugin', namespace),
    client.list('CI', namespace),
  ]);
  return {
    plugins: plugins
      .map(redactPlugin)
      .sort(
        (a, b) =>
          (a.spec.order ?? 100) - (b.spec.order ?? 100) ||
          a.metadata.name.localeCompare(b.metadata.name),
      ),
    runs: buildRuns(cis.map(redactCI)),
  };
}
