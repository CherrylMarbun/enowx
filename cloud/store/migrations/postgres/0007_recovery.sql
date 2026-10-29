-- A member's recovery key: a device of theirs with no token, its public
-- key alone, which the team's keys are sealed to like any other's. The
-- member keeps its private key; with it, a new device gets the keys back
-- when every other device is lost.
ALTER TABLE devices ADD COLUMN recovery INTEGER NOT NULL DEFAULT 0;
