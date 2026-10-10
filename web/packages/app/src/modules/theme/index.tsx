import { ThemeBlueprint } from '@backstage/plugin-app-react';
import {
  createUnifiedTheme,
  palettes,
  UnifiedThemeProvider,
} from '@backstage/theme';
import { createFrontendModule } from '@backstage/frontend-plugin-api';

const felukka = {
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
    primary: { main: felukka.primary, contrastText: felukka.onPrimary },
    secondary: { main: felukka.secondary },
    error: { main: felukka.error },
    background: { default: felukka.background, paper: felukka.surface },
    text: { primary: felukka.onSurface, secondary: felukka.onSurfaceVariant },
    divider: felukka.outline,
    navigation: {
      background: '#090f14',
      indicator: felukka.primary,
      color: felukka.onSurfaceVariant,
      selectedColor: felukka.primary,
      navItem: { hoverBackground: felukka.surfaceHigh },
    },
  },
  defaultPageTheme: 'home',
  fontFamily: '"Manrope", sans-serif',
});

export const felukkaTheme = ThemeBlueprint.make({
  name: 'felukka',
  params: {
    theme: {
      id: 'felukka',
      title: 'felukka',
      variant: 'dark',
      Provider: ({ children }) => (
        <UnifiedThemeProvider theme={theme}>{children}</UnifiedThemeProvider>
      ),
    },
  },
});

export const themeModule = createFrontendModule({
  pluginId: 'app',
  extensions: [felukkaTheme],
});
