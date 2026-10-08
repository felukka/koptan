import { Progress } from '@backstage/core-components';
import {
  Alert,
  ButtonLink,
  Card,
  CardBody,
  CardHeader,
  Flex,
  Grid,
  Text,
} from '@backstage/ui';
import useAsync from 'react-use/esm/useAsync';
import { useKoptanApi } from '../api';
import { Phase } from './Phase';

/** "Raseef": each app's Origin -> Slipway (build) -> Voyage (deploy) chain. */
export const PipelinesPage = () => {
  const { getPipelines } = useKoptanApi();
  const { value, loading, error } = useAsync(getPipelines, [getPipelines]);

  if (loading) return <Progress />;
  if (error) {
    return (
      <Alert
        status="danger"
        title="Could not load pipelines"
        description={error.message}
      />
    );
  }
  return (
    <Flex direction="column" gap="4">
      <Flex justify="end">
        <ButtonLink href="new" variant="primary">
          New deployment
        </ButtonLink>
      </Flex>
      {(value ?? []).length === 0 && (
        <Text color="secondary">No Koptan apps found in the cluster.</Text>
      )}
      {(value ?? []).map(({ app, slipway, voyage }) => (
        <Grid.Root
          key={[
            app.metadata.namespace,
            app.metadata.name,
            slipway?.metadata.name,
            voyage?.metadata.name,
          ].join('/')}
          columns={{ initial: '1', md: '3' }}
          gap="4"
        >
          <Card>
            <CardHeader>
              <Text variant="title-small">
                {app.metadata.name} <Text color="secondary">({app.kind})</Text>
              </Text>
            </CardHeader>
            <CardBody>
              <Phase phase={app.status?.phase} />
              <Text as="div" variant="body-small" color="secondary">
                {app.spec.source.repo} @ {app.spec.source.revision ?? 'main'}
              </Text>
              {app.status?.error && (
                <Text as="div" variant="body-small" color="danger">
                  {app.status.error}
                </Text>
              )}
            </CardBody>
          </Card>
          <Card>
            <CardHeader>
              <Text variant="title-small">Slipway</Text>
            </CardHeader>
            <CardBody>
              {slipway ? (
                <>
                  <Phase phase={slipway.status?.phase} />
                  <Text as="div" variant="body-small" color="secondary">
                    {slipway.status?.latestImage ?? 'No image built yet'}
                  </Text>
                  <Text as="div" variant="body-small" color="secondary">
                    Builds: {slipway.status?.buildCount ?? 0}
                  </Text>
                </>
              ) : (
                <Text color="secondary">No slipway</Text>
              )}
            </CardBody>
          </Card>
          <Card>
            <CardHeader>
              <Text variant="title-small">Voyage</Text>
            </CardHeader>
            <CardBody>
              {voyage ? (
                <>
                  <Phase phase={voyage.status?.phase} />
                  <Text as="div" variant="body-small" color="secondary">
                    Port {voyage.spec.port}, {voyage.spec.replicas ?? 1}{' '}
                    replica(s)
                  </Text>
                  <Text as="div" variant="body-small" color="secondary">
                    {voyage.status?.deployedImage ?? 'Not deployed'}
                  </Text>
                </>
              ) : (
                <Text color="secondary">No voyage</Text>
              )}
            </CardBody>
          </Card>
        </Grid.Root>
      ))}
    </Flex>
  );
};
