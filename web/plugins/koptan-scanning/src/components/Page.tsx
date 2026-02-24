import { Progress } from '@backstage/core-components';
import {
  Icon,
  KoptanPage,
  Section,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import useAsyncRetry from 'react-use/esm/useAsyncRetry';
import { PluginsTable } from './PluginsTable';
import { RunsTable } from './RunsTable';

export const Page = () => {
  const { getScanBay } = useKoptanApi();
  const { value, loading, error, retry } = useAsyncRetry(getScanBay, [
    getScanBay,
  ]);
  const failing =
    value?.runs.filter((r) => r.results.some((s) => s.phase === 'Failed'))
      .length ?? 0;

  return (
    <KoptanPage
      title="Security"
      description={
        <>
          Checks that run after checkout and before every build.{' '}
          {value && (
            <span className={failing ? 'mz-error-text' : 'mz-accent'}>
              {failing
                ? `${failing} service(s) failing a check`
                : 'All checks passing'}
            </span>
          )}
        </>
      }
      actions={
        <button type="button" className="mz-btn" onClick={retry}>
          <Icon name="refresh" size={16} /> Refresh
        </button>
      }
    >
      {loading && <Progress />}
      {error && (
        <div className="mz-alert">
          <strong>Could not load plugins.</strong> {error.message}
        </div>
      )}
      {value && (
        <>
          <Section icon={<Icon name="extension" />} title="Plugins">
            {value.plugins.length ? (
              <PluginsTable plugins={value.plugins} />
            ) : (
              <span className="mz-muted">
                No CIPlugins yet. Apply one (see
                config/samples/koptan_v1_ciplugin_*.yaml) and list it in a
                Service's spec.plugins, or let it target Services itself.
              </span>
            )}
          </Section>
          <Section icon={<Icon name="radar" />} title="Latest results">
            {value.runs.length ? (
              <RunsTable runs={value.runs} />
            ) : (
              <span className="mz-muted">No build has run a plugin yet.</span>
            )}
          </Section>
        </>
      )}
    </KoptanPage>
  );
};
