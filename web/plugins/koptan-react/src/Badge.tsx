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

/** The badge colour for an app kind. */
export const kindVariant = (kind: string): BadgeVariant =>
  (({ GoApp: 'go', JavaApp: 'java', DotnetApp: 'dotnet' })[kind] as
    | BadgeVariant
    | undefined) ?? 'default';
