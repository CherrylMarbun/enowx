-- Devices signing in with a code (api.PathDeviceCode), until a member
-- approves them or the code expires. The device's code is kept hashed.
CREATE TABLE device_codes (
	hash       BLOB PRIMARY KEY,
	user_code  TEXT NOT NULL UNIQUE,
	name       TEXT NOT NULL,
	kind       TEXT NOT NULL,
	public_key BLOB NOT NULL,
	created    TEXT NOT NULL,
	expires    TEXT NOT NULL,
	account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE
);
