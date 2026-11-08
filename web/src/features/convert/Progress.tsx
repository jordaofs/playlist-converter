import type { Conversion } from '../../lib/types';
import { pauseMessage } from './messages';
const titles: Record<string, string> = {
  queued: 'Your playlist is in line',
  reading: 'Getting your tracklist',
  matching: 'Finding the right recordings',
  publishing: 'Building your new playlist',
};
export function Progress({
  job,
  busy,
  onResume,
  onCancel,
}: {
  job: Conversion;
  busy: boolean;
  onResume: () => void;
  onCancel: () => void;
}) {
  return (
    <section className="status-card" aria-label="Conversion progress">
      <div
        className={`status-orbit ${job.pauseReason ? 'paused' : ''}`}
        aria-hidden="true"
      >
        ↗
      </div>
      <p className="eyebrow">{job.pauseReason ? 'ON HOLD' : 'IN PROGRESS'}</p>
      <h2>
        {job.pauseReason
          ? 'Pick up where you left off'
          : (titles[job.status] ?? 'Preparing your playlist')}
      </h2>
      <p role="status">
        {job.pauseReason
          ? pauseMessage(job.pauseReason) ||
            'Your conversion is paused. You can try resuming below.'
          : 'You can keep this page open. We’ll save your place as we go.'}
      </p>
      {job.status === 'publishing' && <p>{job.published} songs added</p>}
      <div className="button-row">
        {job.pauseReason && (
          <button
            className="primary-button compact"
            disabled={busy}
            onClick={onResume}
          >
            Resume conversion <span aria-hidden="true">↗</span>
          </button>
        )}
        <button className="text-button" disabled={busy} onClick={onCancel}>
          Cancel conversion
        </button>
      </div>
    </section>
  );
}
