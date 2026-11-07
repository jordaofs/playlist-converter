import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { robots, seoHead, siteOrigin, sitemap } from './seo';

// Vitest runs from the web package root.
// Whitespace is collapsed because the formatter may wrap long elements.
const html = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8').replace(
  /\s+/g,
  ' ',
);

describe('static landing page', () => {
  it('describes the product without JavaScript', () => {
    expect(html).toMatch(
      /<title> ?Playlist Converter: move playlists between Spotify and YouTube ?<\/title>/,
    );
    expect(html).toMatch(
      /<meta\s+name="description"\s+content="[^"]*Spotify and YouTube[^"]*"/,
    );
    expect(html).toMatch(
      /<noscript>[\s\S]*Playlist Converter[\s\S]*<\/noscript>/,
    );
    expect(html).toContain('<!--seo-head-->');
  });
});

describe('site origin', () => {
  it.each([
    [
      'https://playlist-converter.jfs-cloud.com',
      'https://playlist-converter.jfs-cloud.com',
    ],
    [
      'https://playlist-converter.jfs-cloud.com/',
      'https://playlist-converter.jfs-cloud.com',
    ],
    ['', undefined],
    [undefined, undefined],
    ['http://example.com', undefined],
    ['https://example.com/path', undefined],
    ['https://user:pass@example.com', undefined],
    ['https://example.com/?q=1', undefined],
    ['javascript:alert(1)', undefined],
    ['https://exa"mple.com', undefined],
  ])('%s → %s', (raw, want) => {
    expect(siteOrigin(raw)).toBe(want);
  });
});

describe('generated head and crawler files', () => {
  const origin = 'https://playlist-converter.jfs-cloud.com';
  it('emits canonical and Open Graph tags only with an origin', () => {
    const head = seoHead(origin);
    expect(head).toContain(`<link rel="canonical" href="${origin}/" />`);
    expect(head).toContain(`<meta property="og:url" content="${origin}/" />`);
    expect(head).toContain(
      '<meta property="og:title" content="Playlist Converter" />',
    );
    expect(seoHead(undefined)).toBe('');
  });
  it('lists only public pages in the sitemap', () => {
    const xml = sitemap(origin);
    expect(xml).toContain(`<loc>${origin}/</loc>`);
    expect(xml).toContain(`<loc>${origin}/privacy</loc>`);
    expect(xml).not.toContain('/api');
    expect(sitemap(undefined)).toBeUndefined();
  });
  it('keeps crawlers out of the API', () => {
    expect(robots(origin)).toBe(
      `User-agent: *\nAllow: /\nDisallow: /api/\nSitemap: ${origin}/sitemap.xml\n`,
    );
    expect(robots(undefined)).toBe(
      'User-agent: *\nAllow: /\nDisallow: /api/\n',
    );
  });
});
