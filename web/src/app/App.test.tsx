import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, it, expect, vi } from 'vitest';
import App from './App';
import { session, job } from '../test/fixtures';
import type { Conversion } from '../lib/types';
let current: Conversion;
let connected = structuredClone(session);
let fail = '';
const requests: { path: string; method: string; body: unknown }[] = [];
beforeEach(() => {
  localStorage.clear();
  current = structuredClone(job);
  connected = structuredClone(session);
  fail = '';
  requests.length = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, options?: RequestInit) => {
      const method = options?.method ?? 'GET';
      const body = options?.body ? JSON.parse(String(options.body)) : undefined;
      requests.push({ path, method, body });
      if (fail)
        return {
          ok: false,
          json: async () => ({
            success: false,
            error: { code: 'failed', message: fail },
          }),
        };
      if (path === '/api/session') {
        if (method === 'DELETE')
          connected = { ...session, connections: [], conversionIds: [] };
        return {
          ok: true,
          json: async () => ({ success: true, data: connected }),
        };
      }
      if (path.startsWith('/api/connections/')) {
        connected = { ...connected, connections: [] };
        return { ok: true, json: async () => ({ success: true, data: {} }) };
      }
      if (path.endsWith('/decisions'))
        current = {
          ...current,
          status: 'ready_to_publish',
          revision: 2,
          items: [
            { ...current.items[0], decision: 'manual', selectedId: 'y1' },
          ],
        };
      if (path.endsWith('/publish'))
        current = {
          ...current,
          status: 'completed',
          published: 1,
          destinationUrl: 'https://www.youtube.com/playlist?list=x',
        };
      if (path.endsWith('/cancel'))
        current = { ...current, status: 'cancelled' };
      if (path.endsWith('/resume'))
        current = {
          ...current,
          status: 'ready_to_publish',
          pauseReason: undefined,
        };
      return { ok: true, json: async () => ({ success: true, data: current }) };
    }),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});
it('converts, reviews, publishes, starts again and clears local data', async () => {
  const u = userEvent.setup();
  render(<App />);
  await screen.findByLabelText('Your playlist link');
  await u.type(
    screen.getByLabelText('Your playlist link'),
    'https://open.spotify.com/playlist/abc',
  );
  await u.click(screen.getByRole('button', { name: /Find my songs/ }));
  await screen.findByText('Your songs, lined up.');
  await u.click(screen.getByRole('button', { name: /night drive\s*artist/i }));
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: /Create playlist/ }),
    ).toBeEnabled(),
  );
  await u.click(screen.getByRole('button', { name: /Create playlist/ }));
  await screen.findByText('Same favorites. New home.');
  expect(requests.find((r) => r.path.endsWith('/publish'))?.body).toEqual({
    revision: 2,
    name: 'Late nights · YouTube',
  });
  await u.click(screen.getByRole('button', { name: /Start another/ }));
  await screen.findByLabelText('Your playlist link');
  await u.click(screen.getByRole('button', { name: 'Clear my data' }));
  await u.click(screen.getByRole('button', { name: 'Keep my data' }));
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  await u.click(screen.getByRole('button', { name: 'Clear my data' }));
  await u.click(screen.getAllByRole('button', { name: 'Clear my data' })[1]);
  await waitFor(() =>
    expect(screen.getByRole('link', { name: /Connect Spotify/ })).toBeVisible(),
  );
});
it('restores saved progress and cancels', async () => {
  connected = { ...session, conversionIds: ['job'] };
  localStorage.setItem('playlist-converter-conversion', 'job');
  current = { ...job, status: 'matching' };
  render(<App />);
  await screen.findByText('Finding the right recordings');
  await userEvent.click(
    screen.getByRole('button', { name: 'Cancel conversion' }),
  );
  await screen.findByText('No more changes will be made.');
});
it('resumes partial conversions and disconnects', async () => {
  connected = { ...session, conversionIds: ['job'] };
  current = { ...job, status: 'partial', pauseReason: 'quota_exhausted' };
  render(<App />);
  await screen.findByText('A few songs still to go.');
  await userEvent.click(
    screen.getByRole('button', { name: 'Resume conversion' }),
  );
  await screen.findByText('Your songs, lined up.');
  await userEvent.click(
    screen.getByRole('button', { name: 'Disconnect Spotify' }),
  );
  await screen.findByRole('link', { name: /Connect Spotify/ });
});
it('explains how to fix a Google account without a YouTube channel', async () => {
  window.history.replaceState(null, '', '/?auth=no_channel');
  try {
    render(<App />);
    expect(
      await screen.findByText(
        'Your Google account doesn’t have a YouTube channel yet. Create one on youtube.com, then connect again.',
      ),
    ).toBeVisible();
  } finally {
    window.history.replaceState(null, '', '/');
  }
});
it('ignores unknown auth results, including inherited object keys', async () => {
  window.history.replaceState(null, '', '/?auth=constructor');
  try {
    render(<App />);
    await screen.findByRole('main');
    expect(screen.queryByRole('alert')).toBeNull();
  } finally {
    window.history.replaceState(null, '', '/');
  }
});
it('shows an initial connection failure', async () => {
  fail = 'Server unavailable';
  render(<App />);
  await screen.findByRole('alert');
  expect(screen.getByRole('alert')).toHaveTextContent('Server unavailable');
  expect(screen.getByRole('button', { name: 'Reload page' })).toBeVisible();
});
it('keeps the current review on mutation errors', async () => {
  connected = { ...session, conversionIds: ['job'] };
  render(<App />);
  await screen.findByText('Your songs, lined up.');
  fail = 'Please reconnect';
  await userEvent.click(
    screen.getByRole('button', { name: /night drive\s*artist/i }),
  );
  await screen.findByRole('alert');
  expect(screen.getByRole('alert')).toHaveTextContent('Please reconnect');
  expect(screen.getByText('Your songs, lined up.')).toBeVisible();
});
it('polls active jobs and stops when review is ready', async () => {
  vi.useFakeTimers();
  connected = { ...session, conversionIds: ['job'] };
  current = { ...job, status: 'queued' };
  render(<App />);
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  expect(screen.getByText('Your playlist is in line')).toBeVisible();
  current = job;
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1100);
  });
  expect(screen.getByText('Your songs, lined up.')).toBeVisible();
});
it('explains experimental access and links to privacy without inventing a source URL', async () => {
  render(<App />);
  await screen.findByLabelText('Your playlist link');
  expect(screen.getByRole('link', { name: 'Privacy' })).toHaveAttribute(
    'href',
    '/privacy',
  );
  expect(screen.getByText(/Experimental portfolio project/)).toBeVisible();
  expect(screen.getByText(/provider-approved accounts/)).toBeVisible();
  expect(
    screen.queryByRole('link', { name: 'Source code' }),
  ).not.toBeInTheDocument();
});
it('shows only a valid configured public repository link', async () => {
  vi.stubEnv(
    'VITE_SOURCE_URL',
    'https://github.com/example/playlist-converter',
  );
  const view = render(<App />);
  await screen.findByLabelText('Your playlist link');
  expect(screen.getByRole('link', { name: 'Source code' })).toHaveAttribute(
    'href',
    'https://github.com/example/playlist-converter',
  );
  view.unmount();
  vi.stubEnv('VITE_SOURCE_URL', 'javascript:alert(1)');
  render(<App />);
  await screen.findByLabelText('Your playlist link');
  expect(
    screen.queryByRole('link', { name: 'Source code' }),
  ).not.toBeInTheDocument();
  vi.unstubAllEnvs();
});
it('presents the Playlist Converter brand', async () => {
  render(<App />);
  await screen.findByLabelText('Your playlist link');
  expect(
    screen.getByRole('link', { name: 'Playlist Converter home' }),
  ).toBeVisible();
  expect(document.body.textContent).not.toMatch(/side a/i);
});
