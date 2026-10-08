import { Icon, KoptanPage } from '@internal/plugin-koptan-react';
import { Link, useMatch, useNavigate } from 'react-router-dom';
import { NewDeploymentDrawer } from './NewDeploymentDrawer';
import { PipelinesList, usePipelines } from './PipelinesPage';

/** "The Raseef": pipelines list, with the new deployment drawer on /new. */
export const RaseefRoutes = () => {
  const pipelines = usePipelines();
  const navigate = useNavigate();
  const drawerOpen = !!useMatch('/raseef/new');
  const active = (pipelines.value ?? []).filter(
    (p) => p.voyage?.status?.phase === 'Running',
  ).length;

  return (
    <KoptanPage
      title="The Raseef"
      description="Visualizing managed deployment pipelines across the cluster. Tracking slipway status and voyage stability."
      actions={
        <>
          <Link to="/raseef/new" className="mz-glow">
            <Icon name="add_circle" size={20} /> New Deployment
          </Link>
          <div className="mz-pill">
            <span className="mz-dot mz-dot--pulse" />
            <span className="mz-label">Active Voyages: {active}</span>
          </div>
        </>
      }
    >
      <PipelinesList {...pipelines} />
      {drawerOpen && (
        <NewDeploymentDrawer
          onClose={() => navigate('/raseef')}
          onCreated={() => {
            pipelines.retry();
            navigate('/raseef');
          }}
        />
      )}
    </KoptanPage>
  );
};
