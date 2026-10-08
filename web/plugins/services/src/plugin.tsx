import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/services',
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/TodoPage').then(m => (
        <m.TodoPage />
      )),
  },
});

export const servicesPlugin = createFrontendPlugin({
  pluginId: 'services',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  }
});
