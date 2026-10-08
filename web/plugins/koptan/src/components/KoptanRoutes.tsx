import { Container, Header } from '@backstage/ui';
import { NavLink, Route, Routes } from 'react-router-dom';
import { BayPage } from './BayPage';
import { NewPipelinePage } from './NewPipelinePage';
import { PipelinesPage } from './PipelinesPage';
import { Placeholder } from './Placeholder';

const TABS = [
  { to: '', label: 'Bay', end: true },
  { to: 'pipelines', label: 'Raseef' },
  { to: 'scan', label: 'Scan Bay' },
  { to: 'alerts', label: 'Signal Mast' },
];

export const KoptanRoutes = () => (
  <>
    <Header title="Koptan" />
    <Container>
      <nav style={{ display: 'flex', gap: 16, marginBottom: 16 }}>
        {TABS.map((t) => (
          <NavLink
            key={t.label}
            to={t.to}
            end={t.end}
            style={({ isActive }) => ({ fontWeight: isActive ? 700 : 400 })}
          >
            {t.label}
          </NavLink>
        ))}
      </nav>
      <Routes>
        <Route index element={<BayPage />} />
        <Route path="pipelines" element={<PipelinesPage />} />
        <Route path="pipelines/new" element={<NewPipelinePage />} />
        <Route
          path="scan"
          element={
            <Placeholder
              title="Scan Bay"
              note="Vulnerability scanning is not wired up yet. The operator has no scan data source."
            />
          }
        />
        <Route
          path="alerts"
          element={
            <Placeholder
              title="Signal Mast"
              note="Alert channels are not wired up yet. The operator has no alerting backend."
            />
          }
        />
      </Routes>
    </Container>
  </>
);
