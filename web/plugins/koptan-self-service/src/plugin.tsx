import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/self-service',
    title: 'Self Service',
    icon: <Icon name="auto_awesome" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/Routes').then((m) => <m.Routes />),
  },
});

export const SelfServicePlugin = createFrontendPlugin({
  pluginId: 'self-service',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
