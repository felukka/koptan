import { Alert, Button, Flex, PasswordField, TextField } from '@backstage/ui';
import { APP_KINDS, type AppKind } from '@internal/plugin-koptan-common';
import { type FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useKoptanApi } from '../api';

/** Creates an app, its slipway and its voyage in one go. */
export const NewPipelinePage = () => {
  const { createPipeline } = useKoptanApi();
  const navigate = useNavigate();
  const [kind, setKind] = useState<AppKind>('GoApp');
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [repo, setRepo] = useState('');
  const [revision, setRevision] = useState('main');
  const [patToken, setPatToken] = useState('');
  const [registry, setRegistry] = useState('');
  const [image, setImage] = useState('');
  const [port, setPort] = useState('8080');
  const [replicas, setReplicas] = useState('1');
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      await createPipeline({
        name,
        namespace,
        kind,
        source: { repo, revision, ...(patToken ? { patToken } : {}) },
        slipway: { registry, image },
        voyage: { port: Number(port), replicas: Number(replicas) },
      });
      navigate('..', { relative: 'path' });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit}>
      <Flex direction="column" gap="3" style={{ maxWidth: 560 }}>
        {error && (
          <Alert status="danger" title="Could not create" description={error} />
        )}
        <label>
          Kind{' '}
          <select
            value={kind}
            onChange={(e) => setKind(e.target.value as AppKind)}
          >
            {APP_KINDS.map((k) => (
              <option key={k}>{k}</option>
            ))}
          </select>
        </label>
        <TextField label="Name" value={name} onChange={setName} isRequired />
        <TextField
          label="Namespace"
          value={namespace}
          onChange={setNamespace}
        />
        <TextField
          label="Git repository URL"
          value={repo}
          onChange={setRepo}
          isRequired
        />
        <TextField label="Revision" value={revision} onChange={setRevision} />
        <PasswordField
          label="Access token (private repos)"
          value={patToken}
          onChange={setPatToken}
        />
        <TextField
          label="Image registry"
          value={registry}
          onChange={setRegistry}
          isRequired
        />
        <TextField
          label="Image name"
          value={image}
          onChange={setImage}
          isRequired
        />
        <TextField label="Port" value={port} onChange={setPort} isRequired />
        <TextField label="Replicas" value={replicas} onChange={setReplicas} />
        <Flex gap="2">
          <Button type="submit" variant="primary" isPending={busy}>
            Create
          </Button>
          <Button
            variant="secondary"
            onPress={() => navigate('..', { relative: 'path' })}
          >
            Cancel
          </Button>
        </Flex>
      </Flex>
    </form>
  );
};
