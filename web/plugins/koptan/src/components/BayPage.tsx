import { Progress } from '@backstage/core-components';
import {
  Alert,
  Card,
  CardBody,
  CardHeader,
  Flex,
  Grid,
  Text,
} from '@backstage/ui';
import type { Overview } from '@internal/plugin-koptan-common';
import useAsync from 'react-use/esm/useAsync';
import { useKoptanApi } from '../api';

const Stat = ({
  title,
  total,
  byPhase,
}: {
  title: string;
  total: number;
  byPhase: Record<string, number>;
}) => (
  <Card>
    <CardHeader>
      <Text variant="title-small">{title}</Text>
    </CardHeader>
    <CardBody>
      <Text variant="title-large">{total}</Text>
      {Object.entries(byPhase).map(([phase, n]) => (
        <Text key={phase} as="div" variant="body-small" color="secondary">
          {phase}: {n}
        </Text>
      ))}
    </CardBody>
  </Card>
);

/** "The Bay": a fleet overview of apps, builds and deployments. */
export const BayPage = () => {
  const { getOverview } = useKoptanApi();
  const { value, loading, error } = useAsync(getOverview, [getOverview]);

  if (loading) return <Progress />;
  if (error) {
    return (
      <Alert
        status="danger"
        title="Could not load Koptan"
        description={error.message}
      />
    );
  }
  const o = value as Overview;
  return (
    <Flex direction="column" gap="4">
      <Grid.Root columns={{ initial: '1', md: '4' }} gap="4">
        <Stat title="Apps" total={o.apps.total} byPhase={o.apps.byPhase} />
        <Stat
          title="Slipways"
          total={o.slipways.total}
          byPhase={o.slipways.byPhase}
        />
        <Stat
          title="Voyages"
          total={o.voyages.total}
          byPhase={o.voyages.byPhase}
        />
        <Card>
          <CardHeader>
            <Text variant="title-small">Desired replicas</Text>
          </CardHeader>
          <CardBody>
            <Text variant="title-large">{o.replicas.desired}</Text>
          </CardBody>
        </Card>
      </Grid.Root>
    </Flex>
  );
};
