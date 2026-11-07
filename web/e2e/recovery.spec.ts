import { test, expect } from '@playwright/test';
import { connect, start, review, publish, spotifyURL } from './helpers';
test('reload keeps review choices and keyboard dialog is contained on mobile', async ({
  page,
  request,
}) => {
  await request.post('http://127.0.0.1:8081/__test/scenario', {
    data: { mode: 'normal' },
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await connect(page);
  await start(page, spotifyURL);
  await review(page);
  await page.reload();
  await expect(
    page.getByRole('button', { name: 'Create playlist' }),
  ).toBeEnabled();
  await page.getByRole('button', { name: 'Clear my data' }).click();
  await expect(
    page.getByRole('button', { name: 'Keep my data' }),
  ).toBeFocused();
  await page.keyboard.press('Shift+Tab');
  await expect(
    page
      .getByRole('alertdialog')
      .getByRole('button', { name: 'Clear my data' }),
  ).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(
    page.getByRole('button', { name: 'Keep my data' }),
  ).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('alertdialog')).not.toBeVisible();
  await publish(page);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
test('quota pause is recoverable without losing source entries', async ({
  page,
  request,
}) => {
  await request.post('http://127.0.0.1:8081/__test/scenario', {
    data: { mode: 'quota' },
  });
  await connect(page);
  await start(page, spotifyURL);
  await expect(
    page.getByRole('heading', { name: 'Pick up where you left off' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Resume conversion' }).click();
  await publish(page);
});
for (const mode of ['timeout_create', 'timeout_append']) {
  test(`${mode}: reconciles an accepted write after interrupted response`, async ({
    page,
    request,
  }) => {
    await request.post('http://127.0.0.1:8081/__test/scenario', {
      data: { mode: 'normal' },
    });
    await connect(page);
    await start(page, spotifyURL);
    await review(page);
    await request.post('http://127.0.0.1:8081/__test/scenario', {
      data: { mode },
    });
    await page.getByRole('button', { name: 'Create playlist' }).click();
    await expect(
      page.getByRole('button', { name: 'Resume conversion' }),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Resume conversion' }).click();
    await expect(
      page.getByRole('heading', { name: 'Same favorites. New home.' }),
    ).toBeVisible();
    await expect(page.getByRole('status')).toContainText('3 songs');
  });
}
