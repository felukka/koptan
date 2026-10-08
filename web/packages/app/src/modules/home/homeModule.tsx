import { createFrontendModule } from '@backstage/frontend-plugin-api';
import { HomePageWidgetBlueprint } from '@backstage/plugin-home-react/alpha';
import { MarkdownContent } from '@backstage/core-components';

const content = `
## Welcome to Koptan

Koptan builds and deploys apps straight from a git repo.

- **Bay**: a fleet overview of services, builds and deployments
- **Raseef**: each service's pipeline, and the New deployment form
- **Scan Bay** and **Signal Mast**: coming soon

[Koptan on GitHub](https://github.com/felukka/koptan)
`;

const gettingStartedWidget = HomePageWidgetBlueprint.make({
  name: 'getting-started',
  params: {
    name: 'GettingStarted',
    title: 'Getting Started',
    description: 'An introduction to the Koptan pages',
    components: async () => ({
      Content: () => <MarkdownContent content={content} />,
    }),
  },
});

export const homeModule = createFrontendModule({
  pluginId: 'home',
  extensions: [gettingStartedWidget],
});
