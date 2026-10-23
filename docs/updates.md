# Updates

The app updates itself (`ui/updates.go`), with MyGo's updater: it downloads the new version's signed archive, or a delta from the running version, checks the signature against the public key built into the app, and replaces the app.

## In the app

- A little after it starts (15 s) and every 6 hours, the app asks for a newer version, unless **Check for updates automatically** is off in the update dialog. A version found shows as **Update to x.y.z** in the title bar; nothing else interrupts.
- The chip opens the dialog: the release notes, then **Install update**, **Later** or **Skip this version** (a skipped version shows again only when asked for). Installing shows its progress and can be cancelled; the running app goes on meanwhile.
- **Restart now** runs the new version. Its daemon is of the same version, so the app replaces the local one as it connects (`hostconn`), and the daemon brings the sessions back, agents continuing their conversations. Servers' daemons are upgraded the same way at their next connection.
- **Check for updates** in the command palette asks now and says how it went.

Development builds (`make dev`) and builds without an update feed don't update themselves: the palette says so, and the background checks stay quiet.

## Still to do

Where updates come from isn't set up yet: a signing key pair (`mygo keygen`), `updates.publicKey` and `updates.github` (or `url`) in `cmd/enowx/mygo.json`, and a release built with `MYGO_UPDATER_PRIVATE_KEY` (see MyGo's `docs/updates.md`). Until then `mygo.Updater.Enabled()` is false.
