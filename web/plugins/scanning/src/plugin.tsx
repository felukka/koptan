import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/scan',
    title: 'Security',
    icon: <Icon name="shield" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/Page').then((m) => <m.Page />),
  },
});

export const ScanningPlugin = createFrontendPlugin({
  pluginId: 'scanning',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
