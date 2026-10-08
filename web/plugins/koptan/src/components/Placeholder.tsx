import { Alert } from '@backstage/ui';

/** Stand-in for Minzar views that have no data source in the operator yet. */
export const Placeholder = ({
  title,
  note,
}: {
  title: string;
  note: string;
}) => <Alert status="info" title={title} description={note} />;
