import type { Conversion, Session } from '../lib/types';
export const session: Session = {
  csrfToken: 'csrf',
  connections: [
    { provider: 'spotify', id: 's', name: 'Spotify listener' },
    { provider: 'youtube', id: 'y', name: 'YouTube listener' },
  ],
  limits: { maxItems: 100 },
  conversionIds: [],
};
export const job: Conversion = {
  id: 'job',
  sourceProvider: 'spotify',
  targetProvider: 'youtube',
  sourceName: 'Late nights',
  status: 'needs_review',
  revision: 1,
  published: 0,
  createdAt: '2026-01-01',
  updatedAt: '2026-01-01',
  items: [
    {
      id: '1',
      position: 0,
      source: {
        id: 's1',
        title: 'Night Drive',
        artists: ['Artist'],
        durationMs: 180000,
        url: 'https://open.spotify.com/track/1',
      },
      decision: 'review',
      reason: 'Several recordings',
      candidates: [
        {
          song: {
            id: 'y1',
            title: 'Night Drive',
            artists: ['Artist'],
            durationMs: 180000,
            url: 'https://www.youtube.com/watch?v=y1',
          },
          score: 1,
          reasons: ['Same title'],
        },
        {
          song: {
            id: 'y2',
            title: 'Night Drive live',
            artists: ['Artist'],
            durationMs: 190000,
            url: 'javascript:alert(1)',
            unavailable: 'deleted',
          },
          score: 0,
          reasons: ['Unavailable'],
        },
      ],
    },
  ],
};
