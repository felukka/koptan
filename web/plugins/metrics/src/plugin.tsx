import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/',
    title: 'Deck',
    icon: <Icon name="query_stats" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/Page').then((m) => <m.Page />),
  },
});

export const MetricsPlugin = createFrontendPlugin({
  pluginId: 'metrics',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
