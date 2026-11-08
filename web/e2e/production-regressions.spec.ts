// Browser journeys for the failures seen on the first real conversions
// (2026-11-08). Each mode of the fixture server reproduces one of them; see
// internal/testfixtures/providers/server.go.
import {
  test,
  expect,
  type APIRequestContext,
  type Page,
} from '@playwright/test';
import { connect, review, start, spotifyURL } from './helpers';

const scenario = (request: APIRequestContext, mode: string) =>
  request.post('http://127.0.0.1:8081/__test/scenario', { data: { mode } });

async function latest(page: Page) {
  return page.evaluate(async () => {
    const s = await fetch('/api/session').then((r) => r.json());
    const ids: string[] = s.data.conversionIds;
    return fetch(`/api/conversions/${ids[ids.length - 1]}`).then((r) =>
      r.json(),
    );
  });
}

test('connects when the consent page delivers the callback twice', async ({
  page,
  request,
}) => {
  // The second navigation aborts the first while it exchanges the code.
  await scenario(request, 'duplicate_callback');
  await connect(page);
  await expect(page.getByText('We couldn’t connect your account')).toHaveCount(
    0,
  );
  const session = await page.evaluate(() =>
    fetch('/api/session').then((r) => r.json()),
  );
  expect(
    session.data.connections.map((c: { provider: string }) => c.provider),
  ).toEqual(expect.arrayContaining(['spotify', 'youtube']));
  await scenario(request, 'normal');
});

test('one click selects the best match for every song that needs a choice', async ({
  page,
  request,
}) => {
  await scenario(request, 'normal');
  await connect(page);
  await start(page, spotifyURL);
  await expect(
    page.getByRole('heading', { name: 'Your songs, lined up.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: /Use best match for all/ }).click();
  await expect(page.locator('.match-label.review')).toHaveCount(0);
  await expect(
    page.getByText('You can still change any song below.'),
  ).toHaveCount(0);
  await page.getByRole('button', { name: 'Create playlist' }).click();
  await expect(
    page.getByRole('heading', { name: 'Same favorites. New home.' }),
  ).toBeVisible();
  await expect(page.getByRole('status')).toContainText('3 songs');
  const summary = await latest(page);
  expect(summary.data.published).toBe(3);
  expect(
    summary.data.items.filter(
      (i: { decision: string }) => i.decision === 'review',
    ),
  ).toHaveLength(0);
});

test('finishes on YouTube even when reads trail the writes', async ({
  page,
  request,
}) => {
  test.setTimeout(90_000);
  await scenario(request, 'normal');
  await connect(page);
  await start(page, spotifyURL);
  await review(page);
  // The new playlist is missing on its first read and every song is missing
  // from the read right after its insert, as YouTube did in production.
  await scenario(request, 'lagging');
  await page.getByRole('button', { name: 'Create playlist' }).click();
  await expect(
    page.getByRole('heading', { name: 'Same favorites. New home.' }),
  ).toBeVisible({ timeout: 60_000 });
  await expect(page.getByRole('status')).toContainText('3 songs');
  // Completion means the destination matched exactly: no song was sent twice.
  const summary = await latest(page);
  expect(summary.data.published).toBe(3);
  expect(summary.data.pauseReason).toBeUndefined();
  await scenario(request, 'normal');
});

test('a refused source playlist stops with an explanation instead of a resume loop', async ({
  page,
  request,
}) => {
  await scenario(request, 'normal');
  await connect(page);
  await scenario(request, 'deny_source');
  await start(page, spotifyURL);
  await expect(
    page.getByRole('heading', { name: 'We couldn’t finish this playlist.' }),
  ).toBeVisible();
  await expect(page.getByRole('status')).toHaveText(
    'The music service refused access to this playlist. Only playlists you created with the connected account can be converted.',
  );
  await expect(
    page.getByRole('button', { name: 'Resume conversion' }),
  ).toHaveCount(0);
  await scenario(request, 'normal');
});
