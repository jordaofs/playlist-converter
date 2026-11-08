import { useState } from 'react';
import { providerName, type Conversion, type Decision } from '../../lib/types';
import { safeLink } from '../../lib/api';
function duration(ms: number) {
  return `${Math.floor(ms / 60000)}:${String(Math.floor(ms / 1000) % 60).padStart(2, '0')}`;
}
export function Review({
  job,
  busy,
  onDecision,
  onDecisions,
  onPublish,
  onCancel,
}: {
  job: Conversion;
  busy: boolean;
  onDecision: (decision: Decision) => void;
  onDecisions: (decisions: Decision[]) => void;
  onPublish: (name: string) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(
    job.sourceName
      ? `${job.sourceName} · ${providerName(job.targetProvider)}`
      : 'My transferred playlist',
  );
  const unresolved = job.items.filter(
    (item) => ['review', 'pending'].includes(item.decision) || !item.decision,
  ).length;
  // The top available candidate for every song still waiting for a choice.
  // Candidates arrive sorted by score; the user confirms with one click and
  // can still change any song afterwards.
  const best: Decision[] = job.items.flatMap((item) => {
    if (item.decision && !['review', 'pending'].includes(item.decision))
      return [];
    const pick = (item.candidates ?? []).find(
      (candidate) => !candidate.song.unavailable && candidate.song.id,
    );
    return pick
      ? [{ itemId: item.id, candidateId: pick.song.id, action: 'choose' }]
      : [];
  });
  const selected = job.items.filter(
    (item) =>
      item.selectedId && ['automatic', 'manual'].includes(item.decision),
  ).length;
  return (
    <section className="review-section">
      <div className="section-heading">
        <div>
          <p className="eyebrow">A LITTLE LISTENING CHECK</p>
          <h2>Your songs, lined up.</h2>
        </div>
        <span className="count-pill">{job.items.length} tracks</span>
      </div>
      <p className="section-intro">
        {unresolved
          ? `${unresolved} ${unresolved === 1 ? 'song needs' : 'songs need'} your choice. Pick a recording or skip it.`
          : 'Everything is ready for your final check.'}{' '}
        Your original playlist stays as it is.
      </p>
      {best.length > 0 && (
        <div className="bulk-choice">
          <button
            className="secondary-button"
            disabled={busy}
            onClick={() => onDecisions(best)}
          >
            Use best match for all ({best.length})
          </button>
          <span>You can still change any song below.</span>
        </div>
      )}
      <ol className="track-list">
        {job.items.map((item) => (
          <li key={item.id} className="track-row">
            <div className="track-heading">
              <span className="track-number">
                {String(item.position + 1).padStart(2, '0')}
              </span>
              <div className="track-copy">
                <strong>{item.source.title || 'Unavailable song'}</strong>
                <span>
                  {(item.source.artists ?? []).join(', ') || 'Artist unknown'}
                </span>
              </div>
              <span className={`match-label ${item.decision}`}>
                {(
                  {
                    automatic: 'Matched',
                    manual: 'Your choice',
                    review: 'Your choice needed',
                    skipped: 'Skipped',
                    unavailable: 'Unavailable',
                  } as Record<string, string>
                )[item.decision] ?? 'Needs review'}
              </span>
            </div>
            {item.reason && <p className="match-reason">{item.reason}</p>}
            <details open={item.decision === 'review'}>
              <summary>
                {item.decision === 'review'
                  ? 'Choose a recording'
                  : 'Review alternatives'}
              </summary>
              <div className="candidates">
                {(item.candidates ?? []).map((candidate) => (
                  <div
                    className={`candidate ${item.selectedId === candidate.song.id ? 'selected' : ''}`}
                    key={candidate.song.id}
                  >
                    <button
                      type="button"
                      className="candidate-choice"
                      disabled={busy || !!candidate.song.unavailable}
                      aria-pressed={item.selectedId === candidate.song.id}
                      onClick={() =>
                        onDecision({
                          itemId: item.id,
                          candidateId: candidate.song.id,
                          action: 'choose',
                        })
                      }
                    >
                      <span className="radio-dot" aria-hidden="true" />
                      <span>
                        <strong>{candidate.song.title}</strong>
                        <small>
                          {(candidate.song.artists ?? []).join(', ')} ·{' '}
                          {duration(candidate.song.durationMs)}
                        </small>
                        <small>{candidate.reasons.join(' · ')}</small>
                      </span>
                    </button>
                    {safeLink(candidate.song.url) && (
                      <a
                        href={safeLink(candidate.song.url)}
                        target="_blank"
                        rel="noreferrer"
                        aria-label={`Open ${candidate.song.title} on ${providerName(job.targetProvider)}`}
                      >
                        ↗
                      </a>
                    )}
                  </div>
                ))}
                <button
                  className="text-button"
                  disabled={busy || item.decision === 'skipped'}
                  onClick={() =>
                    onDecision({ itemId: item.id, action: 'skip' })
                  }
                >
                  Skip this song
                </button>
              </div>
            </details>
          </li>
        ))}
      </ol>
      <div className="publish-panel">
        <label htmlFor="playlist-name">New playlist name</label>
        <input
          id="playlist-name"
          value={name}
          maxLength={100}
          onChange={(event) => setName(event.target.value)}
          disabled={busy}
        />
        <div className="publish-footer">
          <span>{selected} songs · Private playlist</span>
          <button
            className="primary-button compact"
            disabled={busy || unresolved > 0 || selected === 0 || !name.trim()}
            onClick={() => onPublish(name.trim())}
          >
            Create playlist <span aria-hidden="true">↗</span>
          </button>
        </div>
        {selected === 0 && !unresolved && (
          <p className="error-text">
            Choose at least one recording to create a playlist.
          </p>
        )}
      </div>
      <button className="text-button" onClick={onCancel} disabled={busy}>
        Cancel conversion
      </button>
    </section>
  );
}
