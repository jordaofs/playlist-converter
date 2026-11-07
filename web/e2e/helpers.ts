import { expect, type Page } from '@playwright/test';
export const spotifyURL =
  'https://open.spotify.com/playlist/0123456789abcdefghijkl';
export const youtubeURL =
  'https://www.youtube.com/playlist?list=PLOwnedYouTube123';
export async function connect(page: Page) {
  await page.goto('/');
  for (const provider of ['Spotify', 'YouTube']) {
    await page.getByRole('link', { name: `Connect ${provider}` }).click();
    await expect(
      page.getByRole('button', { name: `Disconnect ${provider}` }),
    ).toBeVisible();
  }
}
export async function start(page: Page, url: string) {
  await page.getByLabel('Your playlist link').fill(url);
  await page.getByRole('button', { name: 'Find my songs' }).click();
}
export async function review(page: Page) {
  await expect(
    page.getByRole('heading', { name: 'Your songs, lined up.' }),
  ).toBeVisible();
  const unresolved = page
    .locator('.track-row')
    .filter({ has: page.locator('.match-label.review') });
  while (await unresolved.count()) {
    await unresolved
      .first()
      .locator('.candidate-choice:not(:disabled)')
      .first()
      .click();
    await expect(
      page.getByRole('button', { name: 'Create playlist' }),
    ).toBeEnabled();
  }
}
export async function publish(page: Page) {
  await review(page);
  await page.getByRole('button', { name: 'Create playlist' }).click();
  await expect(
    page.getByRole('heading', { name: 'Same favorites. New home.' }),
  ).toBeVisible();
  await expect(page.getByRole('status')).toContainText('3 songs');
  await expect(page.getByRole('link', { name: 'Open playlist' })).toBeVisible();
}
