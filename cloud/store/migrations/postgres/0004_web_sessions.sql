-- A member signed in to the cloud's pages in a browser (approving a
-- device's code), by a cookie whose token is kept hashed.
CREATE TABLE web_sessions (
	hash       BYTEA PRIMARY KEY,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	created    TEXT NOT NULL,
	expires    TEXT NOT NULL
);
