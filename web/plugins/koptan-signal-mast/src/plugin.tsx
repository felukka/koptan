import {
  createFrontendPlugin,
  PageBlueprint,
} from '@backstage/frontend-plugin-api';
import { Icon } from '@internal/plugin-koptan-react';
import { rootRouteRef } from './routes';

export const page = PageBlueprint.make({
  params: {
    path: '/signal-mast',
    title: 'The Signal Mast',
    icon: <Icon name="sensors" />,
    routeRef: rootRouteRef,
    loader: () =>
      import('./components/SignalMastPage').then((m) => <m.SignalMastPage />),
  },
});

export const koptanSignalMastPlugin = createFrontendPlugin({
  pluginId: 'koptan-signal-mast',
  extensions: [page],
  routes: {
    root: rootRouteRef,
  },
});
