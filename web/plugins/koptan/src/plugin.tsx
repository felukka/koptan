import { RiShipLine } from '@remixicon/react';
import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';

import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/koptan',
    title: 'Koptan',
    icon: <RiShipLine />,
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/KoptanRoutes').then((m) => <m.KoptanRoutes />),
  },
});

export const koptanPlugin = createFrontendPlugin({
  pluginId: 'koptan',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
