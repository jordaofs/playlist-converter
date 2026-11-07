import type { Provider } from './types';
export function detectSource(value: string): Provider | null {
  if (value.length > 2048) return null;
  try {
    const u = new URL(value.trim());
    if (u.protocol !== 'https:' || u.username || u.password || u.port)
      return null;
    if (
      u.hostname === 'open.spotify.com' &&
      /^\/playlist\/[a-zA-Z0-9]+\/?$/.test(u.pathname)
    )
      return 'spotify';
    if (
      ['youtube.com', 'www.youtube.com'].includes(u.hostname) &&
      ['/playlist', '/watch'].includes(u.pathname) &&
      u.searchParams.getAll('list').length === 1 &&
      /^[\w-]+$/.test(u.searchParams.get('list') ?? '')
    )
      return 'youtube';
    return null;
  } catch {
    return null;
  }
}
