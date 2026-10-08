import { AppRootElementBlueprint } from '@backstage/frontend-plugin-api';

/** Minzar's thin footer bar along the bottom of the page. */
export const FooterElement = AppRootElementBlueprint.make({
  name: 'footer',
  params: {
    element: <div className="mz-footer">Koptan Console</div>,
  },
});
