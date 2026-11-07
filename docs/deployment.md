# Running the converter

Use Go 1.27.3, Node 24.22.2 and pnpm 12.11.2. Install dependencies with `go mod download` and `cd web && pnpm install --frozen-lockfile`. Build assets with `cd web && pnpm build`, then run `go run ./cmd/server` from the repository root. The server reads environment variables; it does not automatically load `.env`. Export configuration through your shell or use `docker compose up --build`, which reads `.env`.

Copy `.env.example` to `.env` and supply a base64-encoded 32-byte encryption key from your secret manager. Keep this key stable across restarts; changing it makes stored connections unreadable and requires reconnection. Do not commit credentials.

Register Spotify and Google OAuth applications. Enable YouTube Data API v3. Register callbacks exactly as PUBLIC_BASE_URL plus `/api/auth/spotify/callback` and `/api/auth/youtube/callback`. For local Spotify development use the loopback address `127.0.0.1` rather than localhost. Configure app allowlists/test users and both client IDs/secrets. Spotify requests private playlist read/write scopes; Google requests youtube.force-ssl for playlist operations. These permissions grant broader provider capabilities than the UI exposes; the converter only creates destinations.

Open PUBLIC_BASE_URL, connect both accounts, paste an owned playlist URL, review uncertain recordings, and select Create playlist. Browser sessions retain access across reloads. Losing the cookie or allowing the session to expire loses access to its saved conversions; no account recovery is implemented.

Live validation is still required with your registered apps: read an owned playlist, search candidates, explicitly authorize a disposable destination creation, insert songs, and verify order/duplicates in both directions. Offline tests do not establish provider access or real matching accuracy. Default limits are 100 source entries, one active conversion per principal, serialized worker execution, and shared YouTube budgets of 100 searches/10,000 general units per Pacific day.

## Self-hosting behind a reverse proxy

Run one container on any Docker host, publish its port on loopback only (for example `127.0.0.1:8080:8080`) and terminate HTTPS in a reverse proxy such as Caddy or nginx on the same host. Mount a persistent volume at `/data`. Never run more than one instance against the same database: the process takes an exclusive lock and serializes the worker.

Set `APP_ENV=production` and `PUBLIC_BASE_URL` to the exact HTTPS origin, without a path, and register `PUBLIC_BASE_URL/api/auth/{spotify,youtube}/callback` with the providers. Keep `TOKEN_ENCRYPTION_KEY` stable and stored separately, for example in a password manager. Keep secrets in an env file readable only by the deploying user, never in the image or in Git.

Set `TRUSTED_PROXY_CIDRS` to the network the proxy connects from. With Docker port publishing that is the bridge network of the compose project, for example `172.16.0.0/12`. The application then takes the client address for rate limiting from the rightmost `X-Forwarded-For` entry, and only when the direct peer is inside that list. Leave it empty when nothing sits in front of the application. The proxy must overwrite, not append to, any client-supplied `X-Forwarded-For`; in Caddy use `header_up X-Forwarded-For {remote_host}`.

Configure the proxy's access log so it never records query strings (OAuth callbacks carry `code` and `state`), cookies, `Authorization` or `Referer` headers, and truncate client addresses. Publish a privacy page with your own operator, retention and contact details.

Health checks: `GET /healthz` (process) and `GET /readyz` (database). The image has no shell utilities such as curl; probe from the host through the published port.

Build arguments: `VITE_SOURCE_URL` (optional public GitHub repository link in the footer) and `VITE_SITE_URL` (optional canonical HTTPS origin used for `<link rel="canonical">`, Open Graph tags and `sitemap.xml`). Without `VITE_SITE_URL` no canonical URL or sitemap is emitted, so a fork never advertises another site.

## Backup and restore

Back up while the service runs:

```sh
docker compose exec converter converter backup /data/backups/converter-$(date -u +%Y%m%dT%H%M%SZ).db
```

`converter backup <destination>` opens the database read-only beside the serving process, writes a consistent snapshot with SQLite `VACUUM INTO`, verifies it with `PRAGMA integrity_check`, creates it with mode 0600 and refuses to overwrite an existing file. It needs only `DATABASE_PATH`, not OAuth credentials. Expire old copies on a schedule you publish in your privacy page. Copies contain encrypted tokens, so protect them like the live database.

To restore: stop the service, place the backup as the database file in an empty volume owned by UID 10001, keep the original `TOKEN_ENCRYPTION_KEY`, start one instance and check `/readyz` and an existing session. The schema only moves forward: running an older image against a newer database is not supported.

## Go-live checklist

- Build the container, verify non-root runtime, `/readyz`, volume write permissions, restart and a real restore.
- Confirm the proxy forwards the client address and the access log is filtered.
- Register OAuth callbacks and allowed test users; publish a privacy page matching your deployment.
- Run authorized disposable owned-playlist conversions in both directions, checking private destination, duplicates, order and reported omissions.

Live OAuth smoke tests and independent accuracy evaluation remain pending until performed with real accounts.
