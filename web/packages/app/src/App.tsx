import { createApp } from '@backstage/frontend-defaults';
import { navModule } from './modules/nav';
import { themeModule } from './modules/theme';

import MetricsPlugin from '@internal/plugin-koptan-metrics';
import ServicePlugin from '@internal/plugin-koptan-service';
import ScanningPlugin from '@internal/plugin-koptan-scanning';
import SelfServicePlugin from '@internal/plugin-koptan-self-service';
import AlertingPlugin from '@internal/plugin-koptan-alerting';

export default createApp({
  features: [
    MetricsPlugin,
    ServicePlugin,
    ScanningPlugin,
    SelfServicePlugin,
    AlertingPlugin,
    navModule,
    themeModule,
  ],
});
