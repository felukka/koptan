import { expect, test } from '@playwright/test';

// Needs no cluster: data views may show an error alert, but the sidebar,
// the deployment form and the sample pages must always render.
test('Koptan pages are reachable from the sidebar', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Enter' }).click();

  const nav = page.getByRole('navigation', { name: 'sidebar nav' });
  await expect(nav.getByRole('link', { name: 'The Bay' })).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'The Bay', exact: true }).first(),
  ).toBeVisible();

  await nav.getByRole('link', { name: 'The Raseef' }).click();
  await expect(page).toHaveURL(/\/raseef$/);
  await page.getByRole('link', { name: 'New Deployment' }).click();
  await expect(page).toHaveURL(/\/raseef\/new$/);
  await expect(page.getByRole('button', { name: 'Create' })).toBeVisible();
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page).toHaveURL(/\/raseef$/);

  await nav.getByRole('link', { name: 'The Scan Bay' }).click();
  await expect(page.getByText('Sample data').first()).toBeVisible();

  await nav.getByRole('link', { name: 'The Signal Mast' }).click();
  await expect(page.getByText('Sample data').first()).toBeVisible();

  // Defaults that need the (hidden) catalog service.
  await expect(nav.locator('a[href="/home"]')).toBeVisible();
  await expect(nav.getByRole('link', { name: 'Catalog' })).toHaveCount(0);
  await page.goto('/settings');
  await expect(
    page.getByText('Backstage Identity', { exact: true }),
  ).toBeVisible();
  await page.goto('/notifications');
  await expect(
    page.getByRole('heading', { name: /Unread notifications/ }),
  ).toBeVisible();
});
