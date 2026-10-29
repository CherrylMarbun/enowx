-- Every change of a team's items takes the team's next sequence number,
-- so a device asks for what changed after the last it saw.
ALTER TABLE teams ADD COLUMN item_seq BIGINT NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN seq BIGINT NOT NULL DEFAULT 0;
CREATE INDEX items_seq ON items (team_id, seq);
