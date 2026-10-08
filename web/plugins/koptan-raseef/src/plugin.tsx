import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/raseef',
    title: 'The Raseef',
    icon: <Icon name="grid_view" />,
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/RaseefRoutes').then((m) => <m.RaseefRoutes />),
  },
});

export const koptanRaseefPlugin = createFrontendPlugin({
  pluginId: 'koptan-raseef',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
