import type { ReactNode } from 'react';

export type BadgeVariant =
  | 'default'
  | 'primary'
  | 'success'
  | 'warning'
  | 'error'
  | 'go'
  | 'java'
  | 'dotnet';

export const Badge = ({
  variant = 'default',
  children,
}: {
  variant?: BadgeVariant;
  children: ReactNode;
}) => <span className={`mz-badge mz-badge--${variant}`}>{children}</span>;

/** The badge colour for a detected language (Service status.serviceType). */
export const kindVariant = (language: string): BadgeVariant =>
  (({ go: 'go', java: 'java', dotnet: 'dotnet', '.net': 'dotnet' })[
    language.toLowerCase()
  ] as BadgeVariant | undefined) ?? 'default';
