import type { ReactNode } from 'react';
import './minzar.css';
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
