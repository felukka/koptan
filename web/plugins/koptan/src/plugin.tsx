import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/koptan',
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/TodoPage').then(m => (
        <m.TodoPage />
      )),
  },
});

export const koptanPlugin = createFrontendPlugin({
  pluginId: 'koptan',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  }
});
