# 2. An optional cloud, off by default, for teams

Status: accepted

## Context

enowX was built on "no cloud, no accounts": everything runs on the user's machines, hosts are reached over SSH or the user's tailnet, and a user's own data syncs between their own hosts with the app as the hub ([user-data-sync.md](../user-data-sync.md)). A team working together needs what that can't give: who its members are, notes and canvases they share, hosts and sessions others can open or watch, and a way in for a new member. A mobile app, later, needs the same.

## Decision

- A cloud server, `enowx-cloud`, open source in this repository (`cmd/enowx-cloud`), self-hostable as one binary or image. enowX runs its own instance; anyone can run theirs.
- The app and daemons use it only when its address is set (Settings → Cloud). Without it, enowX works as before, with nothing sent anywhere.
- The cloud never holds credentials, not even encrypted: no vault, agent logins, model API keys, session environments, `.env` files, SSH keys or git tokens.
- What a team shares is encrypted on its members' devices with the team's key; the cloud stores ciphertext and the metadata needed to sync it.
- It doesn't relay sessions: watching or working in a session goes over the host's connection (SSH, the tailnet), as today. Agents' conversations are not stored in the cloud; a session is watched live.

## Consequences

- Product principle 5 becomes "No cloud unless you set one up; no telemetry", and "enowX-operated cloud services, accounts" leaves the non-goals.
- Accounts, teams, roles and keys are new code (`cloud/`), kept out of MyGo like `core/`.
- The cloud's API only grows, as the daemon's protocol does; app, daemon and cloud update independently.
- What the cloud stores and doesn't is in [cloud.md](../cloud.md); a change to it is a change to this decision.
