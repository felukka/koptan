import {
  Sidebar,
  SidebarDivider,
  SidebarGroup,
  SidebarItem,
  SidebarScrollWrapper,
  SidebarSpace,
} from '@backstage/core-components';
import { NavContentBlueprint } from '@backstage/plugin-app-react';
import { NotificationsSidebarItem } from '@backstage/plugin-notifications';
import { SidebarSearchModal } from '@backstage/plugin-search';
import { UserSettingsSignInAvatar } from '@backstage/plugin-user-settings';
import { identityApiRef, useApi } from '@backstage/frontend-plugin-api';

import ExitToAppIcon from '@material-ui/icons/ExitToApp';
import SearchIcon from '@material-ui/icons/Search';
import MenuIcon from '@material-ui/icons/Menu';

import { SidebarLogo } from './SidebarLogo';

const MENU_ITEMS = [
  { id: 'page:metrics', enabled: true },
  { id: 'page:self-service', enabled: true },
  { id: 'page:service', enabled: true },
  { id: 'page:scanning', enabled: true },
  { id: 'page:infrastructure', enabled: true },
  { id: 'page:monitoring', enabled: true },
  { id: 'page:alerting', enabled: true },
];

export const SidebarContent = NavContentBlueprint.make({
  params: {
    component: ({ navItems }) => {
      const identityApi = useApi(identityApiRef);

      const nav = navItems.withComponent((item) => (
        <SidebarItem icon={() => item.icon} to={item.href} text={item.title} />
      ));

      nav.take('page:search');
      nav.take('page:notifications');
      nav.take('page:user-settings');

      return (
        <Sidebar>
          <SidebarLogo />
          <SidebarDivider />

          <SidebarGroup label="Search" icon={<SearchIcon />} to="/search">
            <SidebarSearchModal />
          </SidebarGroup>

          <SidebarGroup label="Menu" icon={<MenuIcon />}>
            <SidebarScrollWrapper>
              {MENU_ITEMS.filter((item) => item.enabled).map((item) =>
                nav.take(item.id),
              )}
            </SidebarScrollWrapper>
          </SidebarGroup>

          <SidebarSpace />
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
            {nav.take('page:user-settings')}
          </SidebarGroup>
        </Sidebar>
      );
    },
  },
});
