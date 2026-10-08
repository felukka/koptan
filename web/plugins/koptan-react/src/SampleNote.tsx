import { Badge } from './Badge';

/** Marks a page whose data is illustrative, not read from the cluster. */
export const SampleNote = ({ children }: { children: string }) => (
  <div className="mz-note">
    <Badge variant="warning">Sample data</Badge> {children}
  </div>
);
