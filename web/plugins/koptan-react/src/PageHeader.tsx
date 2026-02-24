import type { ReactNode } from 'react';

/** Big page title, a description line and optional actions on the right. */
export const PageHeader = ({
  title,
  description,
  children,
}: {
  title: string;
  description?: ReactNode;
  children?: ReactNode;
}) => (
  <div className="mz-header">
    <div>
      <h1 className="mz-title">{title}</h1>
      {description && <p className="mz-description">{description}</p>}
    </div>
    {children && <div className="mz-header-actions">{children}</div>}
  </div>
);

/** A titled panel with an icon, as used for Minzar's sections. */
export const Section = ({
  icon,
  title,
  actions,
  children,
}: {
  icon?: ReactNode;
  title: string;
  actions?: ReactNode;
  children: ReactNode;
}) => (
  <section className="mz-panel">
    <div className="mz-panel-head">
      <div className="mz-panel-title">
        {icon}
        <h2 className="mz-label" style={{ margin: 0, color: 'inherit' }}>
          {title}
        </h2>
      </div>
      {actions}
    </div>
    {children}
  </section>
);
