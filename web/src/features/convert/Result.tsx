import { safeLink } from '../../lib/api';
import { providerName, type Conversion } from '../../lib/types';
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
          : job.error ||
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
