import { screen } from '@testing-library/react';
import { renderInTestApp } from '@backstage/frontend-test-utils';
import { TodoPage } from './TodoPage';

describe('TodoPage', () => {
  it('renders the EmptyPlaceholder with correct title', async () => {
    await renderInTestApp(<TodoPage />);

    expect(await screen.findByText('Monitoring')).toBeInTheDocument();
  });

  it('renders a description', async () => {
    await renderInTestApp(<TodoPage />);

    expect(
      await screen.findByText(
        /Observability dashboards and uptime tracking are being wired/i,
      ),
    ).toBeInTheDocument();
  });
});
