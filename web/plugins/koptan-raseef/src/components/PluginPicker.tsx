import { Badge, useKoptanApi } from '@internal/plugin-koptan-react';
import useAsync from 'react-use/esm/useAsync';

/**
 * Checkboxes for the CIPlugins of a namespace. Plugins that attach
 * themselves (targetRefs or selector) run anyway and need no tick.
 */
export const PluginPicker = ({
  namespace,
  value,
  onChange,
}: {
  namespace: string;
  value: string[];
  onChange: (names: string[]) => void;
}) => {
  const { getScanBay } = useKoptanApi();
  const { value: bay, loading, error } = useAsync(getScanBay, [getScanBay]);
  if (loading) return <span className="mz-muted">Loading plugins…</span>;
  if (error) {
    return (
      <span className="mz-muted">Plugins unavailable: {error.message}</span>
    );
  }
  const plugins = (bay?.plugins ?? []).filter(
    (p) => (p.metadata.namespace ?? 'default') === namespace,
  );
  if (!plugins.length) {
    return (
      <span className="mz-muted">No CIPlugins in namespace {namespace}.</span>
    );
  }
  const toggle = (name: string) =>
    onChange(
      value.includes(name) ? value.filter((n) => n !== name) : [...value, name],
    );
  return (
    <div className="mz-list-edit">
      {plugins.map((p) => (
        <label key={p.metadata.name} className="mz-check">
          <input
            type="checkbox"
            checked={value.includes(p.metadata.name)}
            onChange={() => toggle(p.metadata.name)}
          />
          <strong>{p.metadata.name}</strong> <Badge>{p.spec.type}</Badge>
        </label>
      ))}
    </div>
  );
};
