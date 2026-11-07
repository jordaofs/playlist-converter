import { ClearDataDialog } from './ClearDataDialog';
import { useState } from 'react';
import { Connections } from '../features/connect/Connections';
import { PlaylistForm } from '../features/convert/PlaylistForm';
import { Progress } from '../features/convert/Progress';
import { Review } from '../features/convert/Review';
import { Result } from '../features/convert/Result';
import { useConversion } from '../features/convert/useConversion';
export default function App() {
  const sourceURL = import.meta.env.VITE_SOURCE_URL;
  const publicSource =
    typeof sourceURL === 'string' &&
    /^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\/?$/.test(
      sourceURL,
    )
      ? sourceURL
      : undefined;
  const state = useConversion();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const job = state.job;
  const ready =
    job &&
    ['needs_review', 'ready_to_publish'].includes(job.status) &&
    !job.pauseReason;
  const terminal =
    job && ['completed', 'partial', 'failed', 'cancelled'].includes(job.status);
  const connected =
    !!state.session &&
    ['spotify', 'youtube'].every((provider) =>
      state.session!.connections.some((c) => c.provider === provider),
    );
  return (
    <>
      <a className="skip-link" href="#main">
        Skip to converter
      </a>
      <header className="site-header">
        <a className="wordmark" href="/" aria-label="Playlist Converter home">
          <span className="brand-disc" aria-hidden="true" />
          playlist converter<span className="brand-period">.</span>
        </a>
        <span className="header-note">GOOD MUSIC GOES WITH YOU</span>
        <a className="header-link" href="#how-it-works">
          How it works <span aria-hidden="true">↗</span>
        </a>
      </header>
      <main id="main">
        <section className="hero">
          <div className="hero-label">
            <span /> YOUR PLAYLISTS, WITHOUT BORDERS
          </div>
          <h1>
            New place.
            <br />
            Same <em>good music.</em>
          </h1>
          <p className="hero-description">
            Take your favorites from Spotify to YouTube.
            <br className="desktop-break" /> Or the other way around. Every song
            deserves a good home.
          </p>
          <div className="service-route" aria-hidden="true">
            <span className="route-service">
              <i className="spotify-dot" />
              Spotify
            </span>
            <span className="route-arrow">↔</span>
            <span className="route-service">
              <i className="youtube-dot" />
              YouTube
            </span>
          </div>
        </section>
        <div className="converter-card">
          {state.error && (
            <div role="alert" className="error-banner">
              {state.error}
            </div>
          )}
          {state.loading ? (
            <div className="loading-state" role="status">
              Getting your space ready…
            </div>
          ) : !state.session ? (
            <div className="loading-state">
              <h2>We couldn’t connect.</h2>
              <p>Please reload this page to try again.</p>
              <button
                className="secondary-button"
                onClick={() => window.location.reload()}
              >
                Reload page
              </button>
            </div>
          ) : (
            <>
              <Connections
                connections={state.session.connections}
                busy={state.busy}
                onDisconnect={(provider) => void state.disconnect(provider)}
              />
              <div className="card-divider" />
              {!job ? (
                <PlaylistForm
                  connected={connected}
                  busy={state.busy}
                  maxItems={state.session.limits.maxItems}
                  onSubmit={(value) => void state.create(value)}
                />
              ) : ready ? (
                <Review
                  job={job}
                  busy={state.busy}
                  onDecision={(decision) => void state.decide(decision)}
                  onPublish={(name) => void state.publish(name)}
                  onCancel={() => void state.cancel()}
                />
              ) : terminal ? (
                <Result
                  job={job}
                  busy={state.busy}
                  onNew={state.startNew}
                  onResume={() => void state.resume()}
                />
              ) : (
                <Progress
                  job={job}
                  busy={state.busy}
                  onResume={() => void state.resume()}
                  onCancel={() => void state.cancel()}
                />
              )}
            </>
          )}
        </div>
        <section
          className="how-section"
          id="how-it-works"
          aria-label="How it works"
        >
          <div>
            <span>01 / BRING YOUR MUSIC</span>
            <h2>One link. Both worlds.</h2>
            <p>
              Connect your accounts and paste a playlist you own. We’ll know
              where it belongs next.
            </p>
          </div>
          <div>
            <span>02 / MAKE IT YOURS</span>
            <h2>The right version matters.</h2>
            <p>
              We check the recording, not just the name. You choose when a match
              needs a closer look.
            </p>
          </div>
          <div>
            <span>03 / PRESS PLAY</span>
            <h2>A fresh start. Nothing lost.</h2>
            <p>
              Your new playlist starts private. The original stays exactly where
              you left it.
            </p>
          </div>
        </section>
        <div className="closing-note">
          <span aria-hidden="true">✳</span>
          <p>
            Less starting over.
            <br />
            <em>More listening.</em>
          </p>
        </div>
      </main>
      <footer>
        <a className="wordmark small" href="/">
          playlist converter.
        </a>
        <p>Made for the music you keep coming back to.</p>
        <nav className="footer-links" aria-label="Project information">
          <a href="/privacy">Privacy</a>
          {publicSource && (
            <a href={publicSource} target="_blank" rel="noopener noreferrer">
              Source code
            </a>
          )}
        </nav>
        <button
          className="text-button"
          onClick={() => setConfirmDelete(true)}
          disabled={!state.session || state.busy}
        >
          Clear my data
        </button>
      </footer>
      <p className="project-note">
        Experimental portfolio project. Real conversions are limited to
        provider-approved accounts. Review uncertain matches; recording accuracy
        has not been independently measured.
      </p>
      <p className="session-note">
        Your saved progress belongs to this browser session. Clearing cookies or
        letting the session expire removes access.
      </p>
      {confirmDelete && (
        <ClearDataDialog
          onClose={() => setConfirmDelete(false)}
          onConfirm={() => {
            setConfirmDelete(false);
            void state.clearData();
          }}
        />
      )}
    </>
  );
}
