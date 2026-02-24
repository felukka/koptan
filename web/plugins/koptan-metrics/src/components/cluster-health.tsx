import { HomePageWidgetBlueprint } from '@backstage/plugin-home-react/alpha';

export const clusterHealthWidget = HomePageWidgetBlueprint.make({
  name: 'cluster-health',
  params: {
    name: 'ClusterHealth',
    title: 'Cluster Health',
    description: 'A cluster status widget',
    components: async () => ({
      Content: () => <div>nb2a nshoof el denia</div>,
    }),
  },
});
