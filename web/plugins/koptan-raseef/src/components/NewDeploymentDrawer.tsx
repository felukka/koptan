import type { CreatePipelineRequest } from '@internal/plugin-koptan-common';
import {
  EnvEditor,
  FormField as Field,
  Icon,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { type FormEvent, useState } from 'react';
import { PluginPicker } from './PluginPicker';

/** Slide-in form that creates a Service; the operator derives its CI and CD. */
export const NewDeploymentDrawer = ({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) => {
  const { createPipeline } = useKoptanApi();
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [repo, setRepo] = useState('');
  const [revision, setRevision] = useState('');
  const [token, setToken] = useState('');
  const [env, setEnv] = useState<[string, string][]>([]);
  const [registry, setRegistry] = useState('');
  const [imageRepo, setImageRepo] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [replicas, setReplicas] = useState('1');
  const [port, setPort] = useState('8080');
  const [contextDir, setContextDir] = useState('');
  const [plugins, setPlugins] = useState<string[]>([]);
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    if (env.some(([k, v]) => !k.trim() && v)) {
      setError('Every environment variable needs a name.');
      setBusy(false);
      return;
    }
    const vars = env.filter(([k]) => k.trim());
    const image = {
      ...(registry ? { registry: registry.trim() } : {}),
      ...(imageRepo ? { repo: imageRepo.trim() } : {}),
      ...(username ? { username, password } : {}),
    };
    const req: CreatePipelineRequest = {
      name: name.trim(),
      namespace: namespace.trim(),
      repo: repo.trim(),
      ...(revision.trim() ? { revision: revision.trim() } : {}),
      ...(token ? { token } : {}),
      ...(vars.length
        ? { env: vars.map(([k, v]) => ({ name: k.trim(), value: v })) }
        : {}),
      ...(Object.keys(image).length ? { image } : {}),
      replicas: Number(replicas),
      port: Number(port),
      ...(contextDir.trim() ? { contextDir: contextDir.trim() } : {}),
      ...(plugins.length ? { plugins } : {}),
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
            Declare a git repository. Koptan detects the language, builds the
            image and deploys it.
          </p>
          {error && (
            <div className="mz-alert" style={{ marginTop: 16 }}>
              {error}
            </div>
          )}

          <div className="mz-form-section">
            <h2>
              <Icon name="code" /> Service configuration
            </h2>
            <Field label="Service name">
              <input
                className="mz-input"
                aria-label="Name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. zenith-core-api"
                required
              />
            </Field>
            <Field label="Namespace">
              <input
                className="mz-input"
                aria-label="Namespace"
                value={namespace}
                onChange={(e) => setNamespace(e.target.value)}
              />
            </Field>
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
                  placeholder="default branch"
                />
              </Field>
              <Field label="Access token (private repos)">
                <input
                  className="mz-input"
                  aria-label="Access token"
                  type="password"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="••••••••••••"
                  autoComplete="off"
                />
              </Field>
            </div>
            <Field label="Build context (monorepos)">
              <input
                className="mz-input"
                aria-label="Build context"
                value={contextDir}
                onChange={(e) => setContextDir(e.target.value)}
                placeholder="repository root, or e.g. services/api"
              />
            </Field>
            <Field label="Environment variables" group>
              <EnvEditor rows={env} onChange={setEnv} />
            </Field>
          </div>

          <div className="mz-form-section">
            <h2>
              <Icon name="precision_manufacturing" /> Build &amp; deploy
            </h2>
            <div className="mz-grid-2">
              <Field label="Registry">
                <input
                  className="mz-input"
                  aria-label="Registry"
                  value={registry}
                  onChange={(e) => setRegistry(e.target.value)}
                  placeholder="operator default"
                />
              </Field>
              <Field label="Image repository">
                <input
                  className="mz-input"
                  aria-label="Image repository"
                  value={imageRepo}
                  onChange={(e) => setImageRepo(e.target.value)}
                  placeholder={name || 'service name'}
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
              <Field label="Registry password / token">
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
            <div className="mz-grid-2">
              <Field label="Replicas">
                <input
                  className="mz-input"
                  aria-label="Replicas"
                  type="number"
                  min={0}
                  max={50}
                  value={replicas}
                  onChange={(e) => setReplicas(e.target.value)}
                  required
                />
              </Field>
              <Field label="Container port">
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
          </div>

          <div className="mz-form-section">
            <h2>
              <Icon name="radar" /> CI plugins
            </h2>
            <Field label="Run after checkout, before build" group>
              <PluginPicker
                namespace={namespace.trim() || 'default'}
                value={plugins}
                onChange={setPlugins}
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
