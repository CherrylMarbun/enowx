-- Accounts, their devices, teams and what they share (docs/cloud.md).
-- Secrets are kept hashed (tokens, invites) or sealed on members' devices
-- (team keys, items): the server holds no key that opens a team's data.

CREATE TABLE accounts (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	email      TEXT NOT NULL DEFAULT '',
	avatar     TEXT NOT NULL DEFAULT '',
	github_id  BIGINT UNIQUE,
	created    TEXT NOT NULL
);

-- An app install or a host's daemon, with its public key (X25519).
CREATE TABLE devices (
	id         TEXT PRIMARY KEY,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	kind       TEXT NOT NULL CHECK (kind IN ('app', 'host')),
	public_key BYTEA NOT NULL,
	created    TEXT NOT NULL,
	last_seen  TEXT NOT NULL
);

-- A device's bearer tokens, SHA-256 of the token.
CREATE TABLE tokens (
	hash       BYTEA PRIMARY KEY,
	device_id  TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	created    TEXT NOT NULL
);

CREATE TABLE teams (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	-- Members may copy the team's items into their personal space.
	copy_out   INTEGER NOT NULL DEFAULT 0,
	epoch      INTEGER NOT NULL DEFAULT 1,
	created    TEXT NOT NULL
);

CREATE TABLE members (
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	role       TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
	joined     TEXT NOT NULL,
	PRIMARY KEY (team_id, account_id)
);

-- One-time invites, by link or email, SHA-256 of the token.
CREATE TABLE invites (
	hash       BYTEA PRIMARY KEY,
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	role       TEXT NOT NULL CHECK (role IN ('admin', 'member', 'viewer')),
	email      TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL REFERENCES accounts(id),
	created    TEXT NOT NULL,
	expires    TEXT NOT NULL,
	used_by    TEXT REFERENCES accounts(id),
	used       TEXT
);

-- A team's key of an epoch, sealed to one device (cloud/seal).
CREATE TABLE team_keys (
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	epoch      INTEGER NOT NULL,
	device_id  TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	sealed     BYTEA NOT NULL,
	PRIMARY KEY (team_id, epoch, device_id)
);

-- A team's item (a note, a canvas, a skill…), sealed with the team's key
-- of its epoch; kind, size and version are all the server knows of it.
CREATE TABLE items (
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	id         TEXT NOT NULL,
	kind       TEXT NOT NULL,
	version    INTEGER NOT NULL,
	epoch      INTEGER NOT NULL,
	sealed     BYTEA,
	deleted    INTEGER NOT NULL DEFAULT 0,
	updated    TEXT NOT NULL,
	updated_by TEXT NOT NULL REFERENCES devices(id),
	PRIMARY KEY (team_id, id)
);

CREATE TABLE audit (
	id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	at         TEXT NOT NULL,
	team_id    TEXT,
	account_id TEXT,
	action     TEXT NOT NULL,
	detail     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX audit_team ON audit (team_id, at);
