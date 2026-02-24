import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/monitoring',
    title: 'Monitoring',
    icon: <Icon name="monitoring" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/TodoPage').then((m) => <m.TodoPage />),
  },
});

export const monitoringPlugin = createFrontendPlugin({
  pluginId: 'monitoring',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
