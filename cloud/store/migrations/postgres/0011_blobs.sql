-- Files a team's chat sends (images), sealed with the team's key: their
-- content is kept in the server's file store, this their owner and size,
-- to bound what a team keeps and let their sender or an admin delete them.
CREATE TABLE blobs (
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	id         TEXT NOT NULL,
	size       INTEGER NOT NULL,
	created_by TEXT NOT NULL,
	created    TEXT NOT NULL,
	PRIMARY KEY (team_id, id)
);
