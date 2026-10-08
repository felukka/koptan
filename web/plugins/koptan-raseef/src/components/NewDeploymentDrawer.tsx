import {
  APP_KINDS,
  type AppKind,
  type CreatePipelineRequest,
} from '@internal/plugin-koptan-common';
import { Icon, useKoptanApi } from '@internal/plugin-koptan-react';
import { type FormEvent, useState } from 'react';

/** The spec field names differ per language; the labels follow them. */
const KIND_FIELDS: Record<
  AppKind,
  { version: [string, string, string]; entrypoint: [string, string, string] }
> = {
  GoApp: {
    version: ['goVersion', 'Go version', 'e.g. 1.24'],
    entrypoint: ['entrypoint', 'Entrypoint', 'e.g. cmd/main.go'],
  },
  JavaApp: {
    version: ['javaVersion', 'Java version', 'e.g. 21'],
    entrypoint: ['artifactPath', 'Artifact path', 'e.g. target/app.jar'],
  },
  DotnetApp: {
    version: ['sdkVersion', '.NET SDK version', 'e.g. 8.0'],
    entrypoint: ['projectPath', 'Project path', 'e.g. src/Api/Api.csproj'],
  },
};

const Field = ({
  label,
  children,
  group,
}: {
  label: string;
  children: React.ReactNode;
  /** Use for editors with several controls, where a <label> would mislead. */
  group?: boolean;
}) =>
  group ? (
    <div className="mz-form-field">
      <div className="mz-field-label">{label}</div>
      {children}
    </div>
  ) : (
    // biome-ignore lint/a11y/noLabelWithoutControl: the control is passed as children
    <label className="mz-form-field">
      <span className="mz-field-label">{label}</span>
      {children}
    </label>
  );

const ListEditor = ({
  values,
  onChange,
  placeholder,
}: {
  values: string[];
  onChange: (v: string[]) => void;
  placeholder: string;
}) => (
  <div className="mz-list-edit">
    {values.map((v, i) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: rows are positional
      <div className="mz-list-row" key={i}>
        <input
          className="mz-input"
          value={v}
          placeholder={placeholder}
          onChange={(e) =>
            onChange(values.map((x, j) => (j === i ? e.target.value : x)))
          }
        />
        <button
          type="button"
          className="mz-btn"
          aria-label="Remove"
          onClick={() => onChange(values.filter((_, j) => j !== i))}
        >
          <Icon name="close" size={16} />
        </button>
      </div>
    ))}
    <button
      type="button"
      className="mz-btn"
      onClick={() => onChange([...values, ''])}
    >
      <Icon name="add" size={16} /> Add
    </button>
  </div>
);

const EnvEditor = ({
  rows,
  onChange,
}: {
  rows: [string, string][];
  onChange: (v: [string, string][]) => void;
}) => (
  <div className="mz-list-edit">
    {rows.map(([k, v], i) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: rows are positional
      <div className="mz-list-row" key={i}>
        <input
          className="mz-input"
          value={k}
          placeholder="KEY"
          onChange={(e) =>
            onChange(rows.map((r, j) => (j === i ? [e.target.value, r[1]] : r)))
          }
        />
        <input
          className="mz-input"
          value={v}
          placeholder="VALUE"
          onChange={(e) =>
            onChange(rows.map((r, j) => (j === i ? [r[0], e.target.value] : r)))
          }
        />
        <button
          type="button"
          className="mz-btn"
          aria-label="Remove"
          onClick={() => onChange(rows.filter((_, j) => j !== i))}
        >
          <Icon name="close" size={16} />
        </button>
      </div>
    ))}
    <button
      type="button"
      className="mz-btn"
      onClick={() => onChange([...rows, ['', '']])}
    >
      <Icon name="add" size={16} /> Add variable
    </button>
  </div>
);

/** Slide-in form that creates an app, its slipway and its voyage. */
export const NewDeploymentDrawer = ({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) => {
  const { createPipeline } = useKoptanApi();
  const [kind, setKind] = useState<AppKind>('GoApp');
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [repo, setRepo] = useState('');
  const [revision, setRevision] = useState('main');
  const [patToken, setPatToken] = useState('');
  const [advanced, setAdvanced] = useState(false);
  const [version, setVersion] = useState('');
  const [entrypoint, setEntrypoint] = useState('');
  const [buildArgs, setBuildArgs] = useState<string[]>([]);
  const [packages, setPackages] = useState<string[]>([]);
  const [env, setEnv] = useState<[string, string][]>([]);
  const [registry, setRegistry] = useState('');
  const [image, setImage] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [replicas, setReplicas] = useState('1');
  const [port, setPort] = useState('8080');
  const [healthPath, setHealthPath] = useState('');
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const fields = KIND_FIELDS[kind];

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    const appSpec: Record<string, unknown> = {};
    if (version) appSpec[fields.version[0]] = version;
    if (entrypoint) appSpec[fields.entrypoint[0]] = entrypoint;
    const args = buildArgs.filter(Boolean);
    if (args.length) appSpec.buildArgs = args;
    const pkgs = packages.filter(Boolean);
    if (pkgs.length) appSpec.extraPackages = pkgs;
    const vars = env.filter(([k]) => k);
    if (vars.length) appSpec.env = Object.fromEntries(vars);
    const req: CreatePipelineRequest = {
      name,
      namespace,
      kind,
      source: { repo, revision, ...(patToken ? { patToken } : {}) },
      appSpec,
      slipway: {
        registry,
        image,
        ...(username ? { username, password } : {}),
      },
      voyage: {
        port: Number(port),
        replicas: Number(replicas),
        ...(healthPath ? { healthCheckPath: healthPath } : {}),
      },
    };
    try {
      await createPipeline(req);
      onCreated();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <button
        type="button"
        className="mz-scrim"
        aria-label="Close"
        onClick={onClose}
      />
      <aside className="mz-drawer" aria-label="New deployment">
        <form onSubmit={submit}>
          <div className="mz-panel-head">
            <span className="mz-label">Koptan / Raseef</span>
            <button
              type="button"
              className="mz-btn"
              onClick={onClose}
              aria-label="Close"
            >
              <Icon name="close" size={16} />
            </button>
          </div>
          <h1>New Deployment</h1>
          <p className="mz-description">
            Creates an app, its slipway and its voyage in one go.
          </p>
          {error && (
            <div className="mz-alert" style={{ marginTop: 16 }}>
              {error}
            </div>
          )}

          <div className="mz-form-section">
            <h2>
              <Icon name="code" /> App configuration
            </h2>
            <Field label="App name">
              <input
                className="mz-input"
                aria-label="Name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. zenith-core-api"
                required
              />
            </Field>
            <div className="mz-grid-2">
              <Field label="Kind">
                <select
                  className="mz-select"
                  aria-label="Kind"
                  value={kind}
                  onChange={(e) => setKind(e.target.value as AppKind)}
                >
                  {APP_KINDS.map((k) => (
                    <option key={k}>{k}</option>
                  ))}
                </select>
              </Field>
              <Field label="Namespace">
                <input
                  className="mz-input"
                  aria-label="Namespace"
                  value={namespace}
                  onChange={(e) => setNamespace(e.target.value)}
                />
              </Field>
            </div>
            <Field label="Source repository">
              <input
                className="mz-input"
                aria-label="Git repository URL"
                value={repo}
                onChange={(e) => setRepo(e.target.value)}
                placeholder="https://github.com/org/repo"
                required
              />
            </Field>
            <div className="mz-grid-2">
              <Field label="Revision">
                <input
                  className="mz-input"
                  aria-label="Revision"
                  value={revision}
                  onChange={(e) => setRevision(e.target.value)}
                  placeholder="main"
                />
              </Field>
              <Field label="Access token (private repos)">
                <input
                  className="mz-input"
                  aria-label="Access token"
                  type="password"
                  value={patToken}
                  onChange={(e) => setPatToken(e.target.value)}
                  placeholder="••••••••••••"
                  autoComplete="off"
                />
              </Field>
            </div>
            <button
              type="button"
              className="mz-link-btn"
              onClick={() => setAdvanced(!advanced)}
              style={{ textAlign: 'left' }}
            >
              <Icon name={advanced ? 'expand_less' : 'expand_more'} size={16} />{' '}
              {advanced ? 'Hide' : 'Show'} advanced options
            </button>
            {advanced && (
              <>
                <div className="mz-grid-2">
                  <Field label={fields.version[1]}>
                    <input
                      className="mz-input"
                      value={version}
                      onChange={(e) => setVersion(e.target.value)}
                      placeholder={fields.version[2]}
                    />
                  </Field>
                  <Field label={fields.entrypoint[1]}>
                    <input
                      className="mz-input"
                      value={entrypoint}
                      onChange={(e) => setEntrypoint(e.target.value)}
                      placeholder={fields.entrypoint[2]}
                    />
                  </Field>
                </div>
                <Field label="Build arguments" group>
                  <ListEditor
                    values={buildArgs}
                    onChange={setBuildArgs}
                    placeholder="e.g. --build-arg KEY=value"
                  />
                </Field>
                <Field label="Extra packages" group>
                  <ListEditor
                    values={packages}
                    onChange={setPackages}
                    placeholder="e.g. curl, jq, libssl-dev"
                  />
                </Field>
                <Field label="Environment variables" group>
                  <EnvEditor rows={env} onChange={setEnv} />
                </Field>
              </>
            )}
          </div>

          <div className="mz-form-section">
            <h2>
              <Icon name="precision_manufacturing" /> Slipway (CI)
            </h2>
            <div className="mz-grid-2">
              <Field label="Registry URL">
                <input
                  className="mz-input"
                  aria-label="Image registry"
                  value={registry}
                  onChange={(e) => setRegistry(e.target.value)}
                  placeholder="docker.io"
                  required
                />
              </Field>
              <Field label="Image artifact">
                <input
                  className="mz-input"
                  aria-label="Image name"
                  value={image}
                  onChange={(e) => setImage(e.target.value)}
                  placeholder="voyager/zenith-engine"
                  required
                />
              </Field>
            </div>
            <div className="mz-grid-2">
              <Field label="Registry username">
                <input
                  className="mz-input"
                  aria-label="Registry username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="off"
                />
              </Field>
              <Field label="Registry token / secret">
                <input
                  className="mz-input"
                  aria-label="Registry password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••••••"
                  autoComplete="off"
                />
              </Field>
            </div>
          </div>

          <div className="mz-form-section">
            <h2>
              <Icon name="rocket_launch" /> Voyage (CD)
            </h2>
            <div className="mz-grid-2">
              <Field label="Target replicas">
                <input
                  className="mz-input"
                  aria-label="Replicas"
                  type="number"
                  min={0}
                  value={replicas}
                  onChange={(e) => setReplicas(e.target.value)}
                />
              </Field>
              <Field label="Service port">
                <input
                  className="mz-input"
                  aria-label="Port"
                  type="number"
                  min={1}
                  max={65535}
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                  required
                />
              </Field>
            </div>
            <Field label="Health check path">
              <input
                className="mz-input"
                aria-label="Health check path"
                value={healthPath}
                onChange={(e) => setHealthPath(e.target.value)}
                placeholder="/healthz"
              />
            </Field>
          </div>

          <div className="mz-form-actions">
            <button type="submit" className="mz-glow" disabled={busy}>
              <Icon name="add_circle" size={20} /> Create
            </button>
            <button type="button" className="mz-btn" onClick={onClose}>
              Cancel
            </button>
          </div>
        </form>
      </aside>
    </>
  );
};
