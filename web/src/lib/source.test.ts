import { describe, it, expect } from 'vitest';
import { detectSource } from './source';
describe('owned playlist input', () => {
  it('detects canonical Spotify and YouTube playlist URLs', () => {
    expect(detectSource('https://open.spotify.com/playlist/123abc')).toBe(
      'spotify',
    );
    expect(
      detectSource('https://www.youtube.com/playlist?list=PLabc_123'),
    ).toBe('youtube');
    expect(detectSource('https://youtube.com/watch?v=abc&list=PLabc')).toBe(
      'youtube',
    );
  });
  it.each([
    '',
    'not a url',
    'http://open.spotify.com/playlist/abc',
    'https://open.spotify.com.evil.com/playlist/abc',
    'https://user@youtube.com/playlist?list=PLabc',
    'https://youtube.com/watch?v=abc',
    'https://youtube.com/playlist?list=a&list=b',
    'https://youtu.be/abc',
    'https://open.spotify.com/album/abc',
  ])('rejects invalid or unsupported URL %s', (value) =>
    expect(detectSource(value)).toBeNull(),
  );
});
