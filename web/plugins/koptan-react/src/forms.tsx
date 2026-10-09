import type { ReactNode } from 'react';
import { Icon } from './Icon';

/** A labelled form control, in the Minzar drawer style. */
export const FormField = ({
  label,
  children,
  group,
}: {
  label: string;
  children: ReactNode;
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

/** Edits KEY=VALUE rows; empty rows are the caller's to drop. */
export const EnvEditor = ({
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
