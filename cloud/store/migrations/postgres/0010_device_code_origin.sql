-- Where a device's sign-in started, for the page approving it to show:
-- an approval asked for from elsewhere, a phishing link's, stands out.
ALTER TABLE device_codes ADD COLUMN from_addr TEXT NOT NULL DEFAULT '';
ALTER TABLE device_codes ADD COLUMN from_place TEXT NOT NULL DEFAULT '';
