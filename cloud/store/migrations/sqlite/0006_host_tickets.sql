-- A member's one-time ticket into a host of their team (docs/cloud.md,
-- Hosts and sessions): asked by one of their devices, for one host
-- device, redeemed by that host alone, once, within minutes. Kept hashed.
CREATE TABLE host_tickets (
	hash        BLOB PRIMARY KEY,
	team_id     TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	from_device TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	host_device TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	expires     TEXT NOT NULL,
	used        INTEGER NOT NULL DEFAULT 0
);
