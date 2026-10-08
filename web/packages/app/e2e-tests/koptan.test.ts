import { expect, test } from '@playwright/test';

// Needs no cluster: data views may show an error alert, but the shell, tabs
// and placeholder pages must always render.
test('Koptan pages are reachable from the sidebar', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Enter' }).click();

  await page
    .getByRole('navigation', { name: 'sidebar nav' })
    .getByRole('link', { name: 'Koptan', exact: true })
    .click();
  await expect(page).toHaveURL(/\/koptan$/);

  await page.getByRole('link', { name: 'Raseef' }).click();
  await expect(page).toHaveURL(/\/koptan\/pipelines$/);
  await expect(
    page.getByRole('link', { name: 'New deployment' }),
  ).toBeVisible();

  await page.getByRole('link', { name: 'Scan Bay' }).click();
  await expect(
    page.getByText('Vulnerability scanning is not wired'),
  ).toBeVisible();

  await page.getByRole('link', { name: 'Signal Mast' }).click();
  await expect(page.getByText('Alert channels are not wired')).toBeVisible();
});
