import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/monitoring',
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/TodoPage').then(m => (
        <m.TodoPage />
      )),
  },
});

export const monitoringPlugin = createFrontendPlugin({
  pluginId: 'monitoring',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  }
});
