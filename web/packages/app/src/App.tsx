import { createApp } from '@backstage/frontend-defaults';
import catalogPlugin from '@backstage/plugin-catalog/alpha';
import koptanBayPlugin from '@internal/plugin-koptan-bay';
import koptanRaseefPlugin from '@internal/plugin-koptan-raseef';
import koptanScanBayPlugin from '@internal/plugin-koptan-scan-bay';
import koptanSelfServicePlugin from '@internal/plugin-koptan-self-service';
import koptanSignalMastPlugin from '@internal/plugin-koptan-signal-mast';
import { homeModule } from './modules/home';
import { navModule } from './modules/nav';
import { themeModule } from './modules/theme';

export default createApp({
  features: [
    catalogPlugin,
    koptanBayPlugin,
    koptanRaseefPlugin,
    koptanScanBayPlugin,
    koptanSelfServicePlugin,
    koptanSignalMastPlugin,
    navModule,
    homeModule,
    themeModule,
  ],
});
