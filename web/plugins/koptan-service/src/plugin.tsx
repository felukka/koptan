import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/service',
    title: 'Services',
    icon: <Icon name="grid_view" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/Routes').then((m) => <m.Routes />),
  },
});

export const ServicePlugin = createFrontendPlugin({
  pluginId: 'service',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
