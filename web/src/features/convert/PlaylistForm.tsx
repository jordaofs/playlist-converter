import { useState } from 'react';
import { detectSource } from '../../lib/source';
import { providerName } from '../../lib/types';
export function PlaylistForm({
  connected,
  busy,
  maxItems,
  onSubmit,
}: {
  connected: boolean;
  busy: boolean;
  maxItems: number;
  onSubmit: (value: string) => void;
}) {
  const [value, setValue] = useState('');
  const [error, setError] = useState('');
  const source = detectSource(value);
  return (
    <form
      className="playlist-form"
      onSubmit={(event) => {
        event.preventDefault();
        if (!source) {
          setError('Paste a valid Spotify or YouTube playlist link.');
          return;
        }
        if (!connected) {
          setError('Connect both music accounts to continue.');
          return;
        }
        setError('');
        onSubmit(value.trim());
      }}
    >
      <label htmlFor="playlist-url">Your playlist link</label>
      <div className={`input-wrap ${error ? 'invalid' : ''}`}>
        <span aria-hidden="true">↗</span>
        <input
          id="playlist-url"
          type="text"
          inputMode="url"
          autoComplete="off"
          value={value}
          placeholder="Paste a Spotify or YouTube playlist link"
          aria-invalid={!!error}
          aria-describedby="playlist-help playlist-error"
          onChange={(event) => {
            setValue(event.target.value);
            setError('');
          }}
          disabled={busy}
        />
      </div>
      <div className="form-caption">
        <p id="playlist-help">
          {source ? (
            <>
              <strong>{providerName(source)}</strong>
              <span aria-hidden="true"> → </span>
              {providerName(source === 'spotify' ? 'youtube' : 'spotify')}
            </>
          ) : (
            <>A playlist you own. Up to {maxItems} songs.</>
          )}
        </p>
        <span className="private-note">↗ Private by default</span>
      </div>
      {error && (
        <p id="playlist-error" role="alert" className="error-text">
          {error}
        </p>
      )}
      <button className="primary-button" disabled={busy} type="submit">
        {busy ? 'Getting things ready…' : 'Find my songs'}
        <span aria-hidden="true">↗</span>
      </button>
      <p className="form-footnote">
        Review the matches before we create anything.
      </p>
    </form>
  );
}
