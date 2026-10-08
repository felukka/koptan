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

/** Koptan's pages, in the order they appear in the sidebar. */
const KOPTAN_PAGES = [
  'page:koptan-bay',
  'page:koptan-raseef',
  'page:koptan-scan-bay',
  'page:koptan-signal-mast',
];

export const SidebarContent = NavContentBlueprint.make({
  params: {
    component: ({ navItems }) => {
      const identityApi = useApi(identityApiRef);
      const nav = navItems.withComponent((item) => (
        <SidebarItem icon={() => item.icon} to={item.href} text={item.title} />
      ));

      // Rendered by hand below
      nav.take('page:search');
      nav.take('page:notifications');

      return (
        <Sidebar>
          <SidebarLogo />
          <SidebarGroup label="Search" icon={<SearchIcon />} to="/search">
            <SidebarSearchModal />
          </SidebarGroup>
          <SidebarDivider />
          <SidebarGroup label="Koptan" icon={<MenuIcon />}>
            {KOPTAN_PAGES.map((id) => nav.take(id))}
            <SidebarDivider />
            {nav.take('page:home')}
            <SidebarScrollWrapper>
              {nav.rest({ sortBy: 'title' })}
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
            {nav.take('page:app-visualizer')}
            {nav.take('page:user-settings')}
          </SidebarGroup>
        </Sidebar>
      );
    },
  },
});
