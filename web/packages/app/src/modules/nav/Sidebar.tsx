import {
  Sidebar,
  SidebarDivider,
  SidebarGroup,
  SidebarItem,
  SidebarSpace,
} from '@backstage/core-components';
import { NavContentBlueprint } from '@backstage/plugin-app-react';
import { NotificationsSidebarItem } from '@backstage/plugin-notifications';
import { SidebarSearchModal } from '@backstage/plugin-search';
import { UserSettingsSignInAvatar } from '@backstage/plugin-user-settings';
import { identityApiRef, useApi } from '@backstage/frontend-plugin-api';
import ExitToAppIcon from '@material-ui/icons/ExitToApp';
import SearchIcon from '@material-ui/icons/Search';
import BarChartIcon from '@material-ui/icons/BarChart';
import StorageIcon from '@material-ui/icons/Storage';
import NotificationsIcon from '@material-ui/icons/Notifications';
import DnsIcon from '@material-ui/icons/Dns';
import HeartMonitorIcon from '@material-ui/icons/FavoriteBorder';
import { SidebarLogo } from './SidebarLogo';

const PAGES = {
  METRICS: ['page:metrics'],
  SERVICES: ['page:service'],
  SCANNING: ['page:scanning'],
  ALERTING: ['page:alerting'],
  INFRASTRUCTURE: ['page:infrastructure'],
  MONITORING: ['page:monitoring'],
  PLATFORM: ['page:home'],
};

export const SidebarContent = NavContentBlueprint.make({
  params: {
    component: ({ navItems }) => {
      const identityApi = useApi(identityApiRef);
      const nav = navItems.withComponent((item) => (
        <SidebarItem icon={() => item.icon} to={item.href} text={item.title} />
      ));

      nav.take('page:search');
      nav.take('page:notifications');

      return (
        <Sidebar>
          <SidebarLogo />
          <SidebarGroup label="Search" icon={<SearchIcon />} to="/search">
            <SidebarSearchModal />
          </SidebarGroup>
          <SidebarDivider />

          <SidebarGroup label="Metrics" icon={<BarChartIcon />}>
            {PAGES.METRICS.map((id) => nav.take(id))}
          </SidebarGroup>

          <SidebarGroup label="Services" icon={<StorageIcon />}>
            {PAGES.SERVICES.map((id) => nav.take(id))}
          </SidebarGroup>

          <SidebarGroup label="Scanning" icon={<StorageIcon />}>
            {PAGES.SCANNING.map((id) => nav.take(id))}
          </SidebarGroup>

          <SidebarGroup label="Alerting" icon={<NotificationsIcon />}>
            {PAGES.ALERTING.map((id) => nav.take(id))}
          </SidebarGroup>

          <SidebarGroup label="Infrastructure" icon={<DnsIcon />}>
            {PAGES.INFRASTRUCTURE.map((id) => nav.take(id))}
          </SidebarGroup>

          <SidebarGroup label="Monitoring" icon={<HeartMonitorIcon />}>
            {PAGES.MONITORING.map((id) => nav.take(id))}
          </SidebarGroup>

          <SidebarSpace />
          <SidebarDivider />

          {PAGES.PLATFORM.map((id) => nav.take(id))}

          <SidebarDivider />

          <NotificationsSidebarItem />
          <SidebarDivider />

          <SidebarItem
            icon={ExitToAppIcon}
            text="Sign out"
            onClick={() => identityApi.signOut()}
          />

          <SidebarGroup
            label="Settings"
            icon={<UserSettingsSignInAvatar />}
            to="/settings"
          >
            {nav.take('page:app-visualizer')}
            {nav.take('page:user-settings')}
          </SidebarGroup>
        </Sidebar>
      );
    },
  },
});
