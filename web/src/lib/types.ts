export type Provider = 'spotify' | 'youtube';
export type Song = {
  id: string;
  title: string;
  artists: string[];
  durationMs: number;
  url: string;
  unavailable?: string;
};
export type Candidate = { song: Song; score: number; reasons: string[] };
export type Item = {
  id: string;
  position: number;
  source: Song;
  candidates: Candidate[];
  decision: string;
  selectedId?: string;
  reason?: string;
};
export type Conversion = {
  id: string;
  sourceProvider: Provider;
  targetProvider: Provider;
  sourceName: string;
  status: string;
  pauseReason?: string;
  error?: string;
  revision: number;
  items: Item[];
  destinationUrl?: string;
  published: number;
  createdAt: string;
  updatedAt: string;
  nextAttempt?: string;
};
export type Connection = { provider: Provider; id: string; name: string };
export type Session = {
  csrfToken: string;
  connections: Connection[];
  limits: { maxItems: number };
  conversionIds: string[];
};
export type Decision = {
  itemId: string;
  candidateId?: string;
  action: 'choose' | 'skip';
};
export const providerName = (provider: Provider) =>
  provider === 'spotify' ? 'Spotify' : 'YouTube';
