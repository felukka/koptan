import type { FormEvent, ReactNode } from 'react';
import { Icon } from './Icon';

/** A slide-in form: scrim, header, error banner, fields and actions. */
export const Drawer = ({
  label,
  title,
  description,
  error,
  busy,
  submitLabel,
  onSubmit,
  onClose,
  children,
}: {
  /** Breadcrumb above the title, e.g. "Koptan / Raseef". */
  label: string;
  title: string;
  description: ReactNode;
  error?: string;
  busy: boolean;
  submitLabel: string;
  onSubmit: () => void;
  onClose: () => void;
  children: ReactNode;
}) => {
  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit();
  };
  return (
    <>
      <button
        type="button"
        className="mz-scrim"
        aria-label="Close"
        onClick={onClose}
      />
      <aside className="mz-drawer" aria-label={title}>
        <form onSubmit={submit}>
          <div className="mz-panel-head">
            <span className="mz-label">{label}</span>
            <button
              type="button"
              className="mz-btn"
              onClick={onClose}
              aria-label="Close"
            >
              <Icon name="close" size={16} />
            </button>
          </div>
          <h1>{title}</h1>
          <p className="mz-description">{description}</p>
          {error && (
            <div className="mz-alert" style={{ marginTop: 16 }}>
              {error}
            </div>
          )}
          {children}
          <div className="mz-form-actions">
            <button type="submit" className="mz-glow" disabled={busy}>
              <Icon name="add_circle" size={20} /> {submitLabel}
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

/** A titled group of fields inside a Drawer. */
export const FormSection = ({
  icon,
  title,
  children,
}: {
  icon: string;
  title: string;
  children: ReactNode;
}) => (
  <div className="mz-form-section">
    <h2>
      <Icon name={icon} /> {title}
    </h2>
    {children}
  </div>
);
