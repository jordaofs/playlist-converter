import { test, expect } from '@playwright/test';
import { connect, start, publish, spotifyURL, youtubeURL } from './helpers';
for (const [name, url] of [
  ['Spotify to YouTube', spotifyURL],
  ['YouTube to Spotify', youtubeURL],
]) {
  test(`${name}: connects, preserves duplicate occurrences and reviews before publishing`, async ({
    page,
    request,
  }) => {
    await request.post('http://127.0.0.1:8081/__test/scenario', {
      data: { mode: 'normal' },
    });
    await connect(page);
    await start(page, url);
    await publish(page);
    const summary = await page.evaluate(async () => {
      const s = await fetch('/api/session').then((r) => r.json());
      return fetch(`/api/conversions/${s.data.conversionIds[0]}`).then((r) =>
        r.json(),
      );
    });
    expect(summary.data.items).toHaveLength(3);
    expect(summary.data.published).toBe(3);
    expect(summary.data.items[0].selectedId).toBe(
      summary.data.items[2].selectedId,
    );
    expect(summary.data.items[1].decision).toBe('manual');
  });
}
