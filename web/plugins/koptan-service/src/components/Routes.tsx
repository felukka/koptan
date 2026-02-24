import { Icon, KoptanPage } from '@internal/plugin-koptan-react';
import { Link, useMatch, useNavigate } from 'react-router-dom';
import { New } from './New';
import { PipelinesList, usePipelines } from './Page';

export const Routes = () => {
  const pipelines = usePipelines();
  const navigate = useNavigate();
  const drawerOpen = !!useMatch('/service/new');
  const active = (pipelines.value ?? []).filter(
    (p) => p.cd?.status?.phase === 'Running',
  ).length;

  return (
    <KoptanPage
      title="Services"
      description="Visualizing managed deployment pipelines across the cluster. Tracking service, build and deployment status."
      actions={
        <>
          <Link to="/service/new" className="mz-glow">
            <Icon name="add_circle" size={20} /> New Deployment
          </Link>
          <div className="mz-pill">
            <span className="mz-dot mz-dot--pulse" />
            <span className="mz-label">Active Service: {active}</span>
          </div>
        </>
      }
    >
      <PipelinesList {...pipelines} />
      {drawerOpen && (
        <New
          onClose={() => navigate('/service')}
          onCreated={() => {
            pipelines.retry();
            navigate('/service');
          }}
        />
      )}
    </KoptanPage>
  );
};
