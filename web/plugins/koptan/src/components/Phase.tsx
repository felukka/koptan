import { Text } from '@backstage/ui';

const COLOR: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
  Ready: 'success',
  Succeeded: 'success',
  Running: 'success',
  Idle: 'info',
  Pending: 'warning',
  Discovering: 'warning',
  Resolving: 'warning',
  Building: 'warning',
  Waiting: 'warning',
  Deploying: 'warning',
  Failed: 'danger',
};

/** A phase name coloured by health: green, amber, red or blue. */
export const Phase = ({ phase }: { phase?: string }) => (
  <Text weight="bold" color={phase ? (COLOR[phase] ?? 'info') : 'secondary'}>
    {phase ?? 'Unknown'}
  </Text>
);
