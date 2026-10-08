import { createBackend } from '@backstage/backend-defaults';

const backend = createBackend();

backend.add(import('@backstage/plugin-app-backend'));

// auth: guest sign-in for now, see app-config.yaml for adding GitHub
backend.add(import('@backstage/plugin-auth-backend'));
backend.add(import('@backstage/plugin-auth-backend-module-guest-provider'));

// permissions: koptan.pipeline.create is checked by the koptan backend
backend.add(import('@backstage/plugin-permission-backend'));
// See https://backstage.io/docs/permissions/getting-started for how to create your own permission policy
backend.add(
  import('@backstage/plugin-permission-backend-module-allow-all-policy'),
);

// catalog: no pages or entities, but Settings, Home and Search depend on it
backend.add(import('@backstage/plugin-catalog-backend'));

// search, with the Postgres/SQLite engine and the catalog collator
backend.add(import('@backstage/plugin-search-backend'));
backend.add(import('@backstage/plugin-search-backend-module-pg'));
backend.add(import('@backstage/plugin-search-backend-module-catalog'));

// kubernetes plugin
backend.add(import('@backstage/plugin-kubernetes-backend'));

// koptan plugin (reads/creates Koptan CRs via the Kubernetes API)
backend.add(import('@internal/plugin-koptan-backend'));

// user settings plugin
backend.add(import('@backstage/plugin-user-settings-backend'));

// notifications and signals plugins
backend.add(import('@backstage/plugin-notifications-backend'));
backend.add(import('@backstage/plugin-signals-backend'));

backend.start();
