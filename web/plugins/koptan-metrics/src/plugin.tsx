import { createFrontendPlugin } from '@backstage/frontend-plugin-api';
import { rootRouteRef } from './routes';
import { clusterHealthWidget } from './components/cluster-health';

export const MetricsPlugin = createFrontendPlugin({
  pluginId: 'metrics',
  extensions: [clusterHealthWidget],
  routes: {
    root: rootRouteRef,
  },
});
