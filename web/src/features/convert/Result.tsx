import { safeLink } from '../../lib/api';
import { providerName, type Conversion } from '../../lib/types';
// Reasons a conversion stopped for good (`job.error`). Unknown codes fall back
// to a neutral sentence instead of showing the raw code.
const failureMessages: Record<string, string> = {
  access_denied:
    'The music service refused access to this playlist. Only playlists you created with the connected account can be converted.',
  not_found:
    'We couldn’t find this playlist. Check the link and make sure it’s one you created.',
  playlist_not_owned:
    'This playlist belongs to someone else. Only playlists you created can be converted.',
  empty_playlist: 'This playlist has no songs to convert.',
  playlist_too_large:
    'This playlist has more songs than the current limit of 100.',
  source_changed:
    'The playlist changed while we were reading it. Start again to use the latest version.',
  api_not_enabled:
    'The music service isn’t available for this site right now. Please try again later.',
  conversion_failed:
    'Something went wrong on our side. Your original playlist is unchanged.',
};
function failureMessage(code?: string) {
  return code && Object.hasOwn(failureMessages, code)
    ? failureMessages[code]
    : '';
}
export function Result({
  job,
  busy,
  onNew,
  onResume,
}: {
  job: Conversion;
  busy: boolean;
  onNew: () => void;
  onResume: () => void;
}) {
  const completed = job.status === 'completed';
  const partial = job.status === 'partial';
  const link = safeLink(job.destinationUrl);
  const counts = [
    ['Matched', job.items.filter((i) => i.decision === 'automatic').length],
    ['Your choices', job.items.filter((i) => i.decision === 'manual').length],
    ['Skipped', job.items.filter((i) => i.decision === 'skipped').length],
    [
      'Unavailable',
      job.items.filter((i) => i.decision === 'unavailable').length,
    ],
  ];
  return (
    <section className="result-card">
      <div className="result-symbol" aria-hidden="true">
        {completed ? '✓' : partial ? '↗' : '—'}
      </div>
      <p className="eyebrow">
        {completed
          ? 'READY FOR A NEW ROTATION'
          : partial
            ? 'YOUR PROGRESS IS SAVED'
            : 'CONVERSION STOPPED'}
      </p>
      <h2>
        {completed
          ? 'Same favorites. New home.'
          : partial
            ? 'A few songs still to go.'
            : job.status === 'cancelled'
              ? 'No more changes will be made.'
              : 'We couldn’t finish this playlist.'}
      </h2>
      <p role="status">
        {completed
          ? `${job.published} songs are ready on ${providerName(job.targetProvider)}.`
          : failureMessage(job.error) ||
            job.pauseReason ||
            'Your original playlist is unchanged.'}
      </p>
      <dl className="result-counts">
        {counts.map(([label, count]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>{count}</dd>
          </div>
        ))}
      </dl>
      <p className="small-note">
        {job.published} added ·{' '}
        {Math.max(
          0,
          job.items.filter((i) => i.selectedId).length - job.published,
        )}{' '}
        selected songs not added
      </p>
      <div className="button-row">
        {link && (
          <a
            className="primary-button compact"
            href={link}
            target="_blank"
            rel="noreferrer"
          >
            Open playlist <span aria-hidden="true">↗</span>
          </a>
        )}
        {partial && (
          <button
            className="secondary-button"
            disabled={busy}
            onClick={onResume}
          >
            Resume conversion
          </button>
        )}
        <button className="text-button" onClick={onNew}>
          Start another playlist
        </button>
      </div>
    </section>
  );
}
