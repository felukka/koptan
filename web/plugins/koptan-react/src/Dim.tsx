import type { ReactNode } from 'react';

/** Small muted text, used for secondary metadata inside metrics/log rows. */
export const Dim = ({
  children,
  size = 'small',
}: {
  children: ReactNode;
  size?: 'small' | 'medium';
}) => (
  <span className="mz-muted" style={{ fontSize: size === 'small' ? 10 : 12, fontWeight: 400 }}>
    {children}
  </span>
);
