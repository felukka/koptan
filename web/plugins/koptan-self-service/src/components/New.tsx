import type {
  AIProvider,
  CreateSelfServiceRequest,
  GitProvider,
} from '@internal/plugin-koptan-common';
import {
  Drawer,
  FormField as Field,
  FormSection,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { useState } from 'react';

type Mode = 'create' | 'existing';

const input = (
  value: string,
  set: (v: string) => void,
  label: string,
  extra = {},
) => (
  <input
    className="mz-input"
    aria-label={label}
    value={value}
    onChange={(e) => set(e.target.value)}
    {...extra}
  />
);

export const New = ({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: (namespace: string, name: string) => void;
}) => {
  const { createSelfService } = useKoptanApi();
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [mode, setMode] = useState<Mode>('create');
  const [provider, setProvider] = useState<GitProvider>('github');
  const [owner, setOwner] = useState('');
  const [url, setUrl] = useState('');
  const [baseURL, setBaseURL] = useState('');
  const [token, setToken] = useState('');
  const [aiProvider, setAiProvider] = useState<AIProvider>('anthropic');
  const [model, setModel] = useState('claude-opus-5-5');
  const [aiBaseURL, setAiBaseURL] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [port, setPort] = useState('8080');
  const [allowCommands, setAllowCommands] = useState(false);
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError(undefined);
    const repo: CreateSelfServiceRequest['repo'] =
      mode === 'existing'
        ? { mode, url: url.trim(), token }
        : {
            mode,
            provider,
            token,
            ...(owner.trim() ? { owner: owner.trim() } : {}),
            ...(baseURL.trim() ? { baseURL: baseURL.trim() } : {}),
          };
    try {
      const ns = namespace.trim() || 'default';
      await createSelfService({
        name: name.trim(),
        namespace: ns,
        repo,
        ai: {
          provider: aiProvider,
          model: model.trim(),
          ...(aiBaseURL.trim() ? { baseURL: aiBaseURL.trim() } : {}),
          ...(apiKey ? { apiKey } : {}),
        },
        port: Number(port),
        allowCommands,
      });
      onCreated(ns, name.trim());
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Drawer
      label="Self Service"
      title="New Session"
      description="Pair a git repository with an agent. Every prompt becomes a commit that Koptan builds and deploys."
      error={error}
      busy={busy}
      submitLabel="Create"
      onSubmit={submit}
      onClose={onClose}
    >
      <FormSection icon="code" title="Repository">
        <div className="mz-grid-2">
          <Field label="Name">
            {input(name, setName, 'Name', {
              required: true,
              placeholder: 'e.g. shop',
            })}
          </Field>
          <Field label="Namespace">
            {input(namespace, setNamespace, 'Namespace')}
          </Field>
        </div>
        <Field label="Repository" group>
          <div className="mz-grid-2">
            {(['create', 'existing'] as Mode[]).map((m) => (
              <label key={m} className="mz-check">
                <input
                  type="radio"
                  name="repo-mode"
                  checked={mode === m}
                  onChange={() => setMode(m)}
                />
                {m === 'create'
                  ? 'Create a new repository'
                  : 'Use an existing repository'}
              </label>
            ))}
          </div>
        </Field>
        {mode === 'create' ? (
          <>
            <div className="mz-grid-2">
              <Field label="Git host">
                <select
                  className="mz-input"
                  aria-label="Git host"
                  value={provider}
                  onChange={(e) => setProvider(e.target.value as GitProvider)}
                >
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                </select>
              </Field>
              <Field label="Owner (org, user or group)">
                {input(owner, setOwner, 'Owner', {
                  placeholder: "token's account",
                })}
              </Field>
            </div>
            <Field label="API URL (GitHub Enterprise or self-hosted GitLab)">
              {input(baseURL, setBaseURL, 'API URL', {
                placeholder: 'https://api.github.com',
              })}
            </Field>
          </>
        ) : (
          <Field label="Clone URL (https)">
            {input(url, setUrl, 'Clone URL', {
              required: true,
              placeholder: 'https://github.com/org/repo.git',
            })}
          </Field>
        )}
        <Field label="Token (create and push)">
          {input(token, setToken, 'Git token', {
            type: 'password',
            required: true,
            autoComplete: 'off',
          })}
        </Field>
      </FormSection>
      <FormSection icon="psychology" title="Model">
        <div className="mz-grid-2">
          <Field label="Provider">
            <select
              className="mz-input"
              aria-label="AI provider"
              value={aiProvider}
              onChange={(e) => setAiProvider(e.target.value as AIProvider)}
            >
              <option value="anthropic">Anthropic (Claude)</option>
              <option value="openai-compatible">
                OpenAI-compatible (OpenAI, Ollama, vLLM…)
              </option>
            </select>
          </Field>
          <Field label="Model">
            {input(model, setModel, 'Model', { required: true })}
          </Field>
        </div>
        {aiProvider === 'openai-compatible' && (
          <Field label="Base URL">
            {input(aiBaseURL, setAiBaseURL, 'AI base URL', {
              required: true,
              placeholder: 'http://ollama.ollama.svc:11434/v1',
            })}
          </Field>
        )}
        <Field label="API key (not needed for local servers)">
          {input(apiKey, setApiKey, 'API key', {
            type: 'password',
            autoComplete: 'off',
          })}
        </Field>
      </FormSection>
      <FormSection icon="rocket_launch" title="Service">
        <div className="mz-grid-2">
          <Field label="Container port">
            {input(port, setPort, 'Port', {
              type: 'number',
              min: 1,
              max: 65535,
            })}
          </Field>
          <Field label="Agent commands" group>
            <label className="mz-check">
              <input
                type="checkbox"
                checked={allowCommands}
                onChange={(e) => setAllowCommands(e.target.checked)}
              />
              Let the agent run tests and tools in its pod
            </label>
          </Field>
        </div>
      </FormSection>
    </Drawer>
  );
};
