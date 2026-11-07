import { afterEach, it, expect, vi } from 'vitest';
import { api, safeLink, ApiError } from './api';
afterEach(() => vi.unstubAllGlobals());
it('sends cookies, CSRF and idempotency keys and unwraps success', async () => {
  const fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ success: true, data: { id: 1 } }),
  });
  vi.stubGlobal('fetch', fetch);
  expect(
    await api('/api/test', {
      method: 'POST',
      body: { x: 1 },
      csrf: 'csrf',
      key: 'id',
    }),
  ).toEqual({ id: 1 });
  expect(fetch).toHaveBeenCalledWith(
    '/api/test',
    expect.objectContaining({
      credentials: 'same-origin',
      method: 'POST',
      body: '{"x":1}',
      headers: expect.objectContaining({
        'X-CSRF-Token': 'csrf',
        'Idempotency-Key': 'id',
      }),
    }),
  );
  await api('/api/test');
});
it('handles invalid and unsuccessful responses', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: false,
      json: async () => {
        throw new Error('bad');
      },
    }),
  );
  await expect(api('/api')).rejects.toThrow('unexpected response');
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({
        success: false,
        error: { code: 'denied', message: 'Denied' },
      }),
    }),
  );
  await expect(api('/api')).rejects.toEqual(new ApiError('denied', 'Denied'));
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValue({ ok: true, json: async () => ({ success: false }) }),
  );
  await expect(api('/api')).rejects.toThrow('Something went wrong');
});
it('allows only safe provider HTTPS links', () => {
  expect(safeLink('https://open.spotify.com/playlist/abc')).toContain(
    'spotify',
  );
  expect(safeLink('https://www.youtube.com/watch?v=x')).toContain('youtube');
  for (const x of [
    undefined,
    'broken',
    'javascript:alert(1)',
    'https://evil.test',
    'http://youtube.com',
    'https://a:b@youtube.com',
  ])
    expect(safeLink(x)).toBeUndefined();
});
