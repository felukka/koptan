import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/scan-bay',
    title: 'The Scan Bay',
    icon: <Icon name="radar" />,
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/ScanBayPage').then((m) => <m.ScanBayPage />),
  },
});

export const koptanScanBayPlugin = createFrontendPlugin({
  pluginId: 'koptan-scan-bay',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
