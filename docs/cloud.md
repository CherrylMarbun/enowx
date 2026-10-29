# enowX Cloud

The cloud lets a team work in enowX together: its members, their roles, the notes, canvases, skills and templates they share, and the hosts and sessions others can open or watch. It is optional and off by default ([decision 2](decisions/0002-optional-cloud.md)): the app and daemons use it only once signed in, with **Sign in** at the rail's foot or in **Settings → Cloud**. The app signs in to enowX's, https://cloud.enowx.ai; a host can be pointed at another with `enowx-app cloud url`. Before signing in nothing reaches a cloud, and enowX works as it always has.

The server, `enowx-cloud`, is open source (`cmd/enowx-cloud`) and runs anywhere a container does. enowX runs one; a team can run its own.

## What it keeps, and what never

| Kept | How |
|---|---|
| Accounts: name, email, avatar, the GitHub account signed in with | in the clear |
| Devices: an app install or a host's daemon, its name, public key, last seen | in the clear |
| Teams, members, roles, invites, the team's settings | in the clear (an invite's token hashed) |
| The team's key, once per device of each member | sealed to the device's public key |
| Notes, canvases, skills, subagents, commands, templates, presets of the team | sealed with the team's key; the cloud sees an item's kind, size, version and when it changed |
| Hosts shared with the team: which, and which of their projects | names and addresses sealed with the team's key |
| An audit log: sign-ins, invites, roles, hosts shared, sessions taken over | in the clear |

Never, not even encrypted:

- credentials: the vault, agents' logins, model API keys, session environments, `.env` files, SSH keys, git and GitHub tokens;
- agents' conversations (a session is watched live, over the host's connection), and traffic caught by the Traffic tool;
- a member's personal space, and their own preferences (appearance, sounds, terminal font).

## Personal and team

Everything has one owner: the member (personal) or a team. Personal stays as it is, on the member's own devices and synced between them by the app. A team's items live in their own folder (`teams/<id>/` beside `notes/`), sync through the cloud, and stay with the team when their author leaves. Nothing becomes a team's without a clear act: made in the team's space, or moved or shared to it, after a confirmation that names who will see it and a scan for secrets. Copying a team's item into one's personal space is off unless the team's admins allow it.

On a host, the user's own notes and canvases stay in `notes/` and `canvas/`; each team's are in `teams/<id>/notes` and `teams/<id>/canvas`, with the team's name in `teams/<id>/team.json`. The host learns its account's teams from the cloud's answers it relays, and shows only the spaces of the account it is signed in as. Its notes, groups and canvases come in one list, each marked with its team (`team` in the daemon's protocol, `space.list` names the spaces); the rail's space picker (Personal, then each team) says which the Notes and Canvas pages show, and what is made there is that space's. A page goes in its parent's space, and a page is never moved under one of another space.

A session in a team's project reads the team's notes and canvases, and a member's personal notes only when that member attaches them.

## Roles

| Role | Can |
|---|---|
| Owner | everything; transfers ownership, deletes the team |
| Admin | invites and removes members, changes roles below Owner, pairs and unpairs team hosts, sets the team's rules |
| Member | edits the team's items, works in its projects, opens sessions on its hosts, watches others' |
| Viewer | reads the team's items, watches sessions |

## Signing in and joining

- **Members** sign in with GitHub, for now: the app starts a device code and opens the cloud's `/device` page with it, the browser signs in with GitHub (the account is made at the first sign-in), shows which device asks, and the member approves it; the app gets its token. The cloud keeps a browser session of its own for 12 hours (a cookie, hashed in the database), not GitHub's token. Registering an account of its own on the cloud comes later, with the cloud's public release; there is no sign-in by email link or code, and no password.
- **Invites** are a one-time link, or an email when SMTP is set, valid seven days, with the role given. Accepting needs signing in.
- **A host's daemon** keeps the host's sign-in: the cloud's address, the device's X25519 key and its token, in `~/.enowx-app/cloud/` (`0600`), so every host, this computer or a server, is a device of the account, and the token never leaves it. The apps and the command line ask the cloud through it (`cloud.request` in the daemon's protocol). With no address set, nothing reaches a cloud.
- **A host's daemon** signs in with a device code: `enowx-app cloud login` prints a code, the member approves it in the app or the browser, and the daemon gets a token of its own, revocable from the app.

Tokens are random, kept hashed on the server, one per device; signing out or removing a device revokes it.

## Keys

- Every device makes an X25519 key pair; the private key never leaves it (the system's keychain in the app, a `0600` file on a host).
- A team has a random 256-bit key per epoch. Each member device gets it sealed to its public key: X25519 with an ephemeral key, HKDF-SHA256, AES-256-GCM (`cloud/seal`).
- Items are sealed with the team's key of the current epoch (AES-256-GCM), their ID and version as additional data, so the server can't swap one for another.
- A new device of a member gets the team's keys from any device holding them (another of their own, or a teammate's), which seals them to it; the cloud refuses a key for a device that isn't a member's, and never replaces a copy a device already has. A recovery key, which the member keeps, gets them too, for when every device is lost.
- Removing a member starts a new epoch: new items use a key they never had. What they could read before, they may have kept; nothing undoes that.

## Hosts and sessions

Two kinds of hosts serve a team: a **team host**, paired to the team by an admin, which stays when members come and go, and a member's **personal host shared** with the team, project by project, until they take it back. Members reach them as today, over SSH or the tailnet; the cloud only says which hosts and projects a team has, and the daemon checks a member's token and role before letting them in. Sessions in a team's project show to the team (status, watching) unless marked private; taking over one needs its owner's approval. A session runs with the member's own agent account and git identity, lent from their vault for that session.

## API

HTTPS and JSON under `/v1`, a bearer token per device. Like the daemon's protocol, it only grows: an endpoint, field or value once released stays, and `GET /v1/info` says what a server can do, so app, daemon and cloud update independently.

| Endpoint | Since | What |
|---|---|---|
| `GET /healthz` | 0 | the server is up |
| `GET /v1/info` | 0 | the server's version, API version, sign-in methods |
| `GET /v1/me` | 0 | the account and device the token is (`Me`) |
| `GET /v1/devices` | 0 | the account's devices, the asker's marked `current` |
| `DELETE /v1/devices/{id}` | 0 | signs one of the account's devices out: its token, and the team keys sealed to it, go |
| `POST /v1/signout` | 0 | signs the asking device out |
| `POST /v1/auth/device` | 0 | a device starts signing in with a code: its name, kind and public key; gets `deviceCode` (kept) and `userCode` (shown, `XXXX-XXXX`), valid 10 minutes |
| `POST /v1/auth/device/token` | 0 | the device asks for its token every `interval` seconds: `authorization_pending` until approved, then its token, once; `expired_token` after |
| `GET /v1/auth/device/pending/{code}` | 0 | a member signed in sees what waits behind a user code, before approving |
| `POST /v1/auth/device/approve` | 0 | a member signs the device showing a user code in to their account |
| `GET /v1/auth/github?next=` | 0 | page: signs the browser in with GitHub, then goes to `next`, a path of the cloud's |
| `GET /v1/auth/github/callback` | 0 | page: GitHub sending the browser back |
| `GET /device?code=` | 0 | page: what waits behind a code, to approve (`verifyUrlComplete` of a device code); signs in first |
| `POST /v1/teams` | 0 | makes a team, the asker its owner |
| `GET /v1/teams` | 0 | the asker's teams, with their role in each |
| `GET /v1/teams/{team}` | 0 | a team with its members; 404 to anyone not a member |
| `PATCH /v1/teams/{team}` | 0 | renames it, or lets members copy its items out (`copyOut`); admins |
| `DELETE /v1/teams/{team}` | 0 | deletes it and all it holds; its owner |
| `PATCH /v1/teams/{team}/members/{account}` | 0 | changes a member's role (admins, below owner); the owner hands the team over by making another member owner, and becomes an admin |
| `DELETE /v1/teams/{team}/members/{account}` | 0 | removes a member (admins), or leaves (the member; the owner hands the team over first); the team's keys sealed to their devices go and its key moves to a new epoch |
| `POST /v1/teams/{team}/invites` | 0 | invites as admin, member or viewer, valid seven days; its token (`enxi_…`) and link once, to the admin |
| `GET /v1/teams/{team}/invites` | 0 | the invites still open, without their tokens; admins |
| `DELETE /v1/teams/{team}/invites/{invite}` | 0 | withdraws one; admins |
| `POST /v1/invites/accept` | 0 | joins the invite's team with its role: the token, or the link as pasted; works once |
| `GET /v1/teams/{team}/keys` | 0 | the asking device's copies of the team's keys, sealed to it, and the epoch new items use |
| `GET /v1/teams/{team}/devices` | 0 | the members' devices, their public keys and the epochs whose key each holds: who to seal the key for |
| `PUT /v1/teams/{team}/keys/{epoch}` | 0 | copies of an epoch's key sealed to members' devices, from a device that holds it; an epoch nobody holds yet, the current one, is started by an admin's device, which keeps a copy for itself. A copy once kept stays |

Every endpoint but `/healthz` and `/v1/info` needs a device's token (`Authorization: Bearer enxc_…`), given when the device signs in; a request without one, or with one signed out, gets 401. Errors are `{"error": "…"}`. Sign-in, teams, invites, keys, items and hosts come with the phases below; each is recorded here when it lands.

## Running a server

```
docker build -f cmd/enowx-cloud/Dockerfile -t enowx-cloud .
docker run -p 8080:8080 -v enowx-cloud:/data -e ENOWX_CLOUD_URL=https://cloud.example.com enowx-cloud
```

| Variable | What |
|---|---|
| `ENOWX_CLOUD_URL` | the address members reach it at (required in production: links and sign-in callbacks use it) |
| `ENOWX_CLOUD_ADDR` | where it listens, `:8080` by default |
| `ENOWX_CLOUD_DATABASE_URL` | a Postgres URL (`postgres://user:pass@host:5432/db`); unset, SQLite in its folder |
| `ENOWX_CLOUD_DATABASE_SCHEMA` | the Postgres schema its tables go in, made when missing: `enowx_cloud` by default, so it can share a database with other apps |
| `ENOWX_CLOUD_DATA` | its folder, `/data` in the image: SQLite's `cloud.db` when no Postgres is set |
| `ENOWX_CLOUD_GITHUB_CLIENT_ID`, `ENOWX_CLOUD_GITHUB_CLIENT_SECRET` | the GitHub OAuth app members sign in with; its callback is `<ENOWX_CLOUD_URL>/v1/auth/github/callback` |
| `ENOWX_CLOUD_SMTP_URL` | `smtp://user:pass@host:587`, to send invites by email; optional |

It speaks plain HTTP: put it behind a proxy that terminates TLS (Coolify's, Caddy, nginx).

## Phases

0. Foundation: this document, the server's frame and database (Postgres, or SQLite with none), its image.
1. Accounts and teams: sign-in, device codes, teams, invites, roles, keys; Settings → Cloud; `enowx-app cloud` and `team` commands.
2. The team's space: the space picker in the rail, team notes and canvases synced through the cloud, moving and sharing to a team, skills, templates and presets.
3. Hosts and sessions: team hosts and shared hosts, members checked by daemons, watching and taking over sessions, the audit log.
4. Used by the enowX team every day, before it opens to others.
