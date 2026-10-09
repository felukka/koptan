import type { ReactNode } from 'react';
// The Minzar stylesheet, split by concern; tokens first.
import './styles/tokens.css';
import './styles/layout.css';
import './styles/status.css';
import './styles/forms.css';
import { PageHeader } from './PageHeader';

/** Shared page frame: Minzar header above the page content. */
export const KoptanPage = ({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) => (
  <div className="mz-page">
    <PageHeader title={title} description={description}>
      {actions}
    </PageHeader>
    {children}
  </div>
);
