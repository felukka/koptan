import {
  coreServices,
  createBackendPlugin,
} from '@backstage/backend-plugin-api';
import { KubeKoptanClient } from './service/k8s';
import { koptanPermissions } from '@internal/plugin-koptan-common';
import { createRouter } from './service/router';

/** Backend for the Koptan UI: reads and creates Koptan CRs via the Kubernetes API. */
export const koptanPlugin = createBackendPlugin({
  pluginId: 'koptan',
  register(env) {
    env.registerInit({
      deps: {
        config: coreServices.rootConfig,
        httpAuth: coreServices.httpAuth,
        httpRouter: coreServices.httpRouter,
        permissions: coreServices.permissions,
        permissionsRegistry: coreServices.permissionsRegistry,
      },
      async init({
        config,
        httpAuth,
        httpRouter,
        permissions,
        permissionsRegistry,
      }) {
        permissionsRegistry.addPermissions(koptanPermissions);
        httpRouter.use(
          await createRouter(
            {
              client: KubeKoptanClient.fromDefault(
                config.getOptionalString('koptan.kubeContext'),
              ),
              httpAuth,
              permissions,
              namespace: config.getOptionalString('koptan.namespace'),
            },
            { agentKey: config.getOptionalString('koptan.agentKey') },
          ),
        );
      },
    });
  },
});
