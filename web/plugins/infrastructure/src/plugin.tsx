import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/infra',
    title: 'Infrastructure',
    icon: <Icon name="dns" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/TodoPage').then((m) => <m.TodoPage />),
  },
});

export const infrastructurePlugin = createFrontendPlugin({
  pluginId: 'infrastructure',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
