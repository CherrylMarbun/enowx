-- A device that wrote items is signed out like any other: which device
-- last wrote an item is a note, not a reference that keeps it alive.
CREATE TABLE items_new (
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	id         TEXT NOT NULL,
	kind       TEXT NOT NULL,
	version    INTEGER NOT NULL,
	epoch      INTEGER NOT NULL,
	sealed     BLOB,
	deleted    INTEGER NOT NULL DEFAULT 0,
	updated    TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	seq        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (team_id, id)
);
INSERT INTO items_new (team_id, id, kind, version, epoch, sealed, deleted, updated, updated_by, seq)
	SELECT team_id, id, kind, version, epoch, sealed, deleted, updated, updated_by, seq FROM items;
DROP TABLE items;
ALTER TABLE items_new RENAME TO items;
CREATE INDEX items_seq ON items (team_id, seq);
