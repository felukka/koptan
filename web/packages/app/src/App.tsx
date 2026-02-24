import { createApp } from '@backstage/frontend-defaults';
import { homeModule } from './modules/home';
import { navModule } from './modules/nav';
import { themeModule } from './modules/theme';
import AlertingPlugin from '@internal/plugin-alerting';
import catalogPlugin from '@backstage/plugin-catalog/alpha';
import infrastructurePlugin from '@internal/plugin-infrastructure';
import monitoringPlugin from '@internal/plugin-monitoring';
import ScanningPlugin from '@internal/plugin-scanning';
import ServicePlugin from '@internal/plugin-service';

export default createApp({
  features: [
    AlertingPlugin,
    catalogPlugin,
    homeModule,
    infrastructurePlugin,
    monitoringPlugin,
    navModule,
    ScanningPlugin,
    ServicePlugin,
    themeModule,
  ],
});
