import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/',
    title: 'The Bay',
    icon: <Icon name="anchor" />,
    routeRef: rootRouteRef,
    loader: () => import('./components/BayPage').then((m) => <m.BayPage />),
  },
});

export const koptanBayPlugin = createFrontendPlugin({
  pluginId: 'koptan-bay',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
