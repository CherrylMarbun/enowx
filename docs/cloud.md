# enowX Cloud

The cloud lets a team work in enowX together: its members, their roles, the notes, canvases, skills and templates they share, and the hosts and sessions others can open or watch. It is optional and off by default ([decision 2](decisions/0002-optional-cloud.md)): the app and daemons use it only when its address is set, in **Settings → Cloud**. Without it, enowX works as it always has.

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

A session in a team's project reads the team's notes and canvases, and a member's personal notes only when that member attaches them.

## Roles

| Role | Can |
|---|---|
| Owner | everything; transfers ownership, deletes the team |
| Admin | invites and removes members, changes roles below Owner, pairs and unpairs team hosts, sets the team's rules |
| Member | edits the team's items, works in its projects, opens sessions on its hosts, watches others' |
| Viewer | reads the team's items, watches sessions |

## Signing in and joining

- **Members** sign in with GitHub. When the server has SMTP set, an emailed sign-in link works too.
- **Invites** are a one-time link, or an email when SMTP is set, valid seven days, with the role given. Accepting needs signing in.
- **A host's daemon** signs in with a device code: `enowx-app cloud login` prints a code, the member approves it in the app or the browser, and the daemon gets a token of its own, revocable from the app.

Tokens are random, kept hashed on the server, one per device; signing out or removing a device revokes it.

## Keys

- Every device makes an X25519 key pair; the private key never leaves it (the system's keychain in the app, a `0600` file on a host).
- A team has a random 256-bit key per epoch. Each member device gets it sealed to its public key: X25519 with an ephemeral key, HKDF-SHA256, AES-256-GCM (`cloud/seal`).
- Items are sealed with the team's key of the current epoch (AES-256-GCM), their ID and version as additional data, so the server can't swap one for another.
- A new device of a member gets the team's keys from another of their devices, or from an admin's device, which approves it. A recovery key, which the member keeps, gets them too, for when every device is lost.
- Removing a member starts a new epoch: new items use a key they never had. What they could read before, they may have kept; nothing undoes that.

## Hosts and sessions

Two kinds of hosts serve a team: a **team host**, paired to the team by an admin, which stays when members come and go, and a member's **personal host shared** with the team, project by project, until they take it back. Members reach them as today, over SSH or the tailnet; the cloud only says which hosts and projects a team has, and the daemon checks a member's token and role before letting them in. Sessions in a team's project show to the team (status, watching) unless marked private; taking over one needs its owner's approval. A session runs with the member's own agent account and git identity, lent from their vault for that session.

## API

HTTPS and JSON under `/v1`, a bearer token per device. Like the daemon's protocol, it only grows: an endpoint, field or value once released stays, and `GET /v1/info` says what a server can do, so app, daemon and cloud update independently.

| Endpoint | Since | What |
|---|---|---|
| `GET /healthz` | 0 | the server is up |
| `GET /v1/info` | 0 | the server's version, API version, sign-in methods, features |

Accounts, devices, teams, invites, keys, items and hosts come with the phases below; each is recorded here when it lands.

## Running a server

```
docker build -f cmd/enowx-cloud/Dockerfile -t enowx-cloud .
docker run -p 8080:8080 -v enowx-cloud:/data -e ENOWX_CLOUD_URL=https://cloud.example.com enowx-cloud
```

| Variable | What |
|---|---|
| `ENOWX_CLOUD_URL` | the address members reach it at (required in production: links and sign-in callbacks use it) |
| `ENOWX_CLOUD_ADDR` | where it listens, `:8080` by default |
| `ENOWX_CLOUD_DATA` | its folder, `/data` in the image: the database (`cloud.db`, SQLite) |
| `ENOWX_CLOUD_GITHUB_CLIENT_ID`, `ENOWX_CLOUD_GITHUB_CLIENT_SECRET` | the GitHub OAuth app members sign in with |
| `ENOWX_CLOUD_SMTP_URL` | `smtp://user:pass@host:587`, for emailed sign-in links and invites; optional |

It speaks plain HTTP: put it behind a proxy that terminates TLS (Coolify's, Caddy, nginx).

## Phases

0. Foundation: this document, the server's frame and database, its image.
1. Accounts and teams: sign-in, device codes, teams, invites, roles, keys; Settings → Cloud; `enowx-app cloud` and `team` commands.
2. The team's space: the space picker in the rail, team notes and canvases synced through the cloud, moving and sharing to a team, skills, templates and presets.
3. Hosts and sessions: team hosts and shared hosts, members checked by daemons, watching and taking over sessions, the audit log.
4. Used by the enowX team every day, before it opens to others.
