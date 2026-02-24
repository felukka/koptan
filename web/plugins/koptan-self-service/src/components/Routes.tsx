import { Icon, KoptanPage } from '@internal/plugin-koptan-react';
import { Link, useMatch, useNavigate } from 'react-router-dom';
import { New } from './New';
import { SessionList, useSelfServices } from './List';
import { Page } from './Page';

export const Routes = () => {
  const sessions = useSelfServices();
  const navigate = useNavigate();
  const drawerOpen = !!useMatch('/self-service/new');
  const session = useMatch('/self-service/:namespace/:name');

  if (session?.params.namespace && session.params.name) {
    return (
      <Page namespace={session.params.namespace} name={session.params.name} />
    );
  }
  return (
    <KoptanPage
      title="Self Service"
      description="Describe what you want; an agent writes it into a git repository, and Koptan builds and deploys every change."
      actions={
        <Link to="/self-service/new" className="mz-glow">
          <Icon name="add_circle" size={20} /> New session
        </Link>
      }
    >
      <SessionList {...sessions} />
      {drawerOpen && (
        <New
          onClose={() => navigate('/self-service')}
          onCreated={(ns, name) => {
            sessions.retry();
            navigate(`/self-service/${ns}/${name}`);
          }}
        />
      )}
    </KoptanPage>
  );
};
