import { providerName, type Connection, type Provider } from '../../lib/types';
export function Connections({
  connections,
  busy,
  onDisconnect,
}: {
  connections: Connection[];
  busy: boolean;
  onDisconnect: (provider: Provider) => void;
}) {
  return (
    <section aria-label="Music accounts" className="connections">
      {(['spotify', 'youtube'] as const).map((provider) => {
        const connection = connections.find((c) => c.provider === provider);
        return (
          <div className="connection" key={provider}>
            <span className={`service-icon ${provider}`} aria-hidden="true">
              {provider === 'spotify' ? '≋' : '▶'}
            </span>
            <div className="connection-copy">
              <strong>{providerName(provider)}</strong>
              <span>
                {connection ? connection.name : 'Connect your account'}
              </span>
            </div>
            {connection ? (
              <button
                className="text-button"
                disabled={busy}
                onClick={() => onDisconnect(provider)}
                aria-label={`Disconnect ${providerName(provider)}`}
              >
                Disconnect
              </button>
            ) : (
              <a
                className="connect-button"
                href={`/api/auth/${provider}/start`}
              >
                Connect{' '}
                <span className="sr-only">{providerName(provider)}</span>
                <span aria-hidden="true">↗</span>
              </a>
            )}
          </div>
        );
      })}
    </section>
  );
}
