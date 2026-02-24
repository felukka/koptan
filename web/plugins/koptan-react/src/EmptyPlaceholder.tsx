import type { ReactNode } from 'react';
import { Icon } from './Icon';
import './minzar.css';

/** An empty-state card shown in skeleton category pages until backend wiring is complete. */
export const EmptyPlaceholder = ({
  icon,
  title,
  description,
}: {
  icon?: string;
  title: string;
  description?: ReactNode;
}) => (
  <div className="mz-empty">
    <div className="mz-empty-icon">
      <Icon name={icon ?? 'construction'} size={40} />
    </div>
    <h2 className="mz-empty-title">{title}</h2>
    {description && <p className="mz-empty-desc">{description}</p>}
  </div>
);
