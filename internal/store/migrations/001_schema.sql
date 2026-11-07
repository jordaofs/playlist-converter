CREATE TABLE IF NOT EXISTS principals(id TEXT PRIMARY KEY, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY, principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE, csrf TEXT NOT NULL, expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS connections(principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE, provider TEXT NOT NULL, identity_id TEXT NOT NULL, name TEXT NOT NULL, token BLOB NOT NULL, PRIMARY KEY(principal_id,provider));
CREATE TABLE IF NOT EXISTS oauth_attempts(state TEXT PRIMARY KEY, principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE, session_id TEXT NOT NULL, provider TEXT NOT NULL, verifier TEXT NOT NULL, expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS conversions(id TEXT PRIMARY KEY, principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE, source_identity TEXT NOT NULL, target_identity TEXT NOT NULL, data BLOB NOT NULL, updated_at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS conversions_owner ON conversions(principal_id);
CREATE TABLE IF NOT EXISTS operations(conversion_id TEXT PRIMARY KEY REFERENCES conversions(id) ON DELETE CASCADE, kind TEXT NOT NULL, position INTEGER NOT NULL, target_id TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS idempotency(principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE, scope TEXT NOT NULL, key TEXT NOT NULL, payload TEXT NOT NULL, conversion_id TEXT NOT NULL REFERENCES conversions(id) ON DELETE CASCADE, PRIMARY KEY(principal_id,scope,key));
CREATE TABLE IF NOT EXISTS quotas(bucket TEXT NOT NULL, day TEXT NOT NULL, used INTEGER NOT NULL, PRIMARY KEY(bucket,day));
