-- The GitHub login an account signed in with, to show; github_id stays
-- what identifies it, as a login can be renamed.
ALTER TABLE accounts ADD COLUMN github_login TEXT NOT NULL DEFAULT '';
