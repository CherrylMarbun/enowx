-- Each epoch's key check (seal.KeyCheck), set by the admin's device that
-- started it: a device compares the key a copy holds with it, so a copy a
-- member sealed with another key is told apart.
CREATE TABLE team_key_checks (
	team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	epoch   INTEGER NOT NULL,
	key_check   BYTEA NOT NULL,
	PRIMARY KEY (team_id, epoch)
);
