import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/alert',
    title: 'Alerts',
    icon: <Icon name="notifications" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/Page').then((m) => <m.Page />),
  },
});

export const AlertingPlugin = createFrontendPlugin({
  pluginId: 'alerting',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
