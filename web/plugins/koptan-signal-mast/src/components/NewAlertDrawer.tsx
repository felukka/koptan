import { ALERT_EVENTS, type AlertEvent } from '@internal/plugin-koptan-common';
import {
  Drawer,
  FormField as Field,
  FormSection,
  useKoptanApi,
} from '@internal/plugin-koptan-react';
import { useState } from 'react';
import { type ChannelDraft, ChannelEditor } from './ChannelEditor';

/** Parses "team=payments, tier=api" into match labels. */
const parseLabels = (text: string) =>
  Object.fromEntries(
    text
      .split(',')
      .map((pair) => pair.split('=').map((s) => s.trim()))
      .filter(([k]) => k),
  );

/** Slide-in form that creates an Alert; channel URLs go to a Secret. */
export const NewAlertDrawer = ({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) => {
  const { createAlert } = useKoptanApi();
  const [name, setName] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [service, setService] = useState('');
  const [labels, setLabels] = useState('');
  const [events, setEvents] = useState<AlertEvent[]>([]);
  const [channels, setChannels] = useState<ChannelDraft[]>([
    { name: 'ops', type: 'slack', url: '' },
  ]);
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const toggle = (e: AlertEvent) =>
    setEvents(
      events.includes(e) ? events.filter((x) => x !== e) : [...events, e],
    );

  const submit = async () => {
    setBusy(true);
    setError(undefined);
    const selector = parseLabels(labels);
    try {
      await createAlert({
        name: name.trim(),
        namespace: namespace.trim(),
        ...(service.trim() ? { service: service.trim() } : {}),
        ...(Object.keys(selector).length ? { selector } : {}),
        ...(events.length ? { events } : {}),
        channels: channels.map((c) => ({
          ...c,
          name: c.name.trim(),
          url: c.url.trim(),
        })),
      });
      onCreated();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Drawer
      label="Koptan / Signal Mast"
      title="New Alert"
      description="Send a Service's pushes, builds and deployments to Slack, Teams or a webhook."
      error={error}
      busy={busy}
      submitLabel="Create"
      onSubmit={submit}
      onClose={onClose}
    >
      <FormSection icon="sensors" title="What to watch">
        <div className="mz-grid-2">
          <Field label="Alert name">
            <input
              className="mz-input"
              aria-label="Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. payments-ops"
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
        </div>
        <div className="mz-grid-2">
          <Field label="Service">
            <input
              className="mz-input"
              aria-label="Service"
              value={service}
              onChange={(e) => setService(e.target.value)}
              placeholder="one Service by name"
            />
          </Field>
          <Field label="Or Services with labels">
            <input
              className="mz-input"
              aria-label="Labels"
              value={labels}
              onChange={(e) => setLabels(e.target.value)}
              placeholder="team=payments"
            />
          </Field>
        </div>
        <Field label="Events (none ticked = all)" group>
          <div className="mz-grid-2">
            {ALERT_EVENTS.map((e) => (
              <label key={e} className="mz-check">
                <input
                  type="checkbox"
                  checked={events.includes(e)}
                  onChange={() => toggle(e)}
                />
                {e}
              </label>
            ))}
          </div>
        </Field>
      </FormSection>
      <FormSection icon="campaign" title="Where to send">
        <ChannelEditor rows={channels} onChange={setChannels} />
      </FormSection>
    </Drawer>
  );
};
