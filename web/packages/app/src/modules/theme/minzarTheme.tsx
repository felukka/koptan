import { ThemeBlueprint } from '@backstage/plugin-app-react';
import {
  createUnifiedTheme,
  palettes,
  UnifiedThemeProvider,
} from '@backstage/theme';
import { createFrontendModule } from '@backstage/frontend-plugin-api';

// Minzar's palette (see koptan-ui/src/styles/globals.css).
const minzar = {
  primary: '#eac07d',
  onPrimary: '#432c00',
  background: '#0e1419',
  surface: '#161c22',
  surfaceHigh: '#242b30',
  onSurface: '#dde3ea',
  onSurfaceVariant: '#c3c7ce',
  secondary: '#b1c9e0',
  tertiary: '#a9c9ef',
  error: '#ffb4ab',
  outline: '#43474d',
};

const theme = createUnifiedTheme({
  palette: {
    ...palettes.dark,
    primary: { main: minzar.primary, contrastText: minzar.onPrimary },
    secondary: { main: minzar.secondary },
    error: { main: minzar.error },
    background: { default: minzar.background, paper: minzar.surface },
    text: { primary: minzar.onSurface, secondary: minzar.onSurfaceVariant },
    divider: minzar.outline,
    navigation: {
      background: '#090f14',
      indicator: minzar.primary,
      color: minzar.onSurfaceVariant,
      selectedColor: minzar.primary,
      navItem: { hoverBackground: minzar.surfaceHigh },
    },
  },
  defaultPageTheme: 'home',
  fontFamily: '"Manrope", sans-serif',
});

export const minzarTheme = ThemeBlueprint.make({
  name: 'minzar',
  params: {
    theme: {
      id: 'minzar',
      title: 'Minzar',
      variant: 'dark',
      Provider: ({ children }) => (
        <UnifiedThemeProvider theme={theme}>{children}</UnifiedThemeProvider>
      ),
    },
  },
});

export const themeModule = createFrontendModule({
  pluginId: 'app',
  extensions: [minzarTheme],
});
