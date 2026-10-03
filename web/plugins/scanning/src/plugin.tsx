import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/scanning',
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/TodoPage').then(m => (
        <m.TodoPage />
      )),
  },
});

export const scanningPlugin = createFrontendPlugin({
  pluginId: 'scanning',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  }
});
