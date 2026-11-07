# Architecture

A React/TypeScript/Tailwind frontend and Go net/http API share one origin. The Go process serves built assets and runs one serialized background worker. SQLite stores atomic JSON conversion snapshots and separate authentication, operation-intent, idempotency and quota records. A process file lock enforces one replica.

## Conversion lifecycle

Validate an owned source URL, connect provider identities, import every occurrence, retrieve/hydrate candidates, rank conservatively, review unresolved songs, freeze the approved selection, create a private destination, append serially and verify readback. Preserve repeated songs and order; a provider ID is not a unique source occurrence.

Provider adapters expose explicit read/search/create/append/inspect capabilities. Matching preserves version qualifiers and abstains on ambiguity. Unknown metadata is not evidence of equivalence. Accepted precision, automatic coverage and publication completion are separate measurements. The checked-in dataset is synthetic and intentionally cannot pass the independent-data release gate.

## Failure recovery

Persist an operation intent before remote mutations. A timed-out response may follow a successful write; reconcile the remote playlist against ordered expected occurrences before replay. Ambiguous results pause for recovery. Local idempotency cannot guarantee remote exactly-once execution. Cancellation drains serialized in-flight work and stops future operations without deleting the destination.

## Security and deployment

Session cookies are HttpOnly, SameSite=Lax and Secure for HTTPS. OAuth state/PKCE bind consent to the initiating session. Tokens are encrypted at rest with authenticated encryption and a stable external key. Connection identity is pinned to jobs. Mutations require canonical Origin and CSRF token. Provider errors are sanitized.

Deploy one process with persistent /data storage, fixed canonical HTTPS origin and separately stored credentials. No testserver is included in the production image. A single SQLite worker favors simplicity and safe recovery over throughput. Multi-replica execution would require a different coordination/storage design.
