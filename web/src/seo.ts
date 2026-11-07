// Build-time SEO helpers. The canonical origin comes from VITE_SITE_URL so a
// self-hosted copy never advertises the maintainer's domain by accident.

const ORIGIN = /^https:\/\/[a-z0-9.-]+(:\d{1,5})?$/;
const PAGES = ['/', '/privacy'];

export function siteOrigin(raw?: string): string | undefined {
  if (!raw) return undefined;
  try {
    const url = new URL(raw);
    const clean =
      url.username === '' &&
      url.password === '' &&
      url.search === '' &&
      url.hash === '' &&
      (url.pathname === '' || url.pathname === '/');
    return clean && ORIGIN.test(url.origin) ? url.origin : undefined;
  } catch {
    return undefined;
  }
}

export function seoHead(origin?: string): string {
  if (!origin) return '';
  return [
    `<link rel="canonical" href="${origin}/" />`,
    '<meta property="og:type" content="website" />',
    '<meta property="og:title" content="Playlist Converter" />',
    '<meta property="og:description" content="Move playlists between Spotify and YouTube, reviewing uncertain matches before anything is created." />',
    `<meta property="og:url" content="${origin}/" />`,
  ].join('\n    ');
}

export function sitemap(origin?: string): string | undefined {
  if (!origin) return undefined;
  const urls = PAGES.map((p) => `  <url><loc>${origin}${p}</loc></url>`);
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls.join('\n')}\n</urlset>\n`;
}

export function robots(origin?: string): string {
  const rules = 'User-agent: *\nAllow: /\nDisallow: /api/\n';
  return origin ? `${rules}Sitemap: ${origin}/sitemap.xml\n` : rules;
}
