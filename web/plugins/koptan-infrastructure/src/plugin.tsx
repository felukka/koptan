import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/infra',
    routeRef: rootRouteRef,
    loader: () => import('./components/TodoPage').then((m) => <m.TodoPage />),
  },
});

export const koptanInfrastructurePlugin = createFrontendPlugin({
  pluginId: 'infrastructure',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
