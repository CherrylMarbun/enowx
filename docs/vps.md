# Creating a server

**Create a server…** on the Hosts page makes a server at a cloud provider with the user's own account and API token, then adds it to the hosts and connects, which installs enowX there (`core/vps`, `ui/vpsform.go`). enowX operates nothing in between: the app talks to the provider's API itself.

| Provider | API | Image | Token |
|---|---|---|---|
| Hetzner Cloud | `api.hetzner.cloud/v1`: `locations`, `server_types` (sizes offered in the region, without deprecated ones, with their gross monthly price), `ssh_keys`, `servers` | `ubuntu-24.04` | Console → project → Security → API tokens, Read & Write |
| DigitalOcean | `api.digitalocean.com/v2`: `regions` (available ones), `sizes` (available in the region), `account/keys`, `droplets` | `ubuntu-24-04-x64` | API → Personal access tokens, with write access |
| Vultr | `api.vultr.com/v2`: `regions`, `plans?type=vc2` (offered in the region), `ssh-keys`, `os` (Ubuntu 24.04 x64), `instances` | Ubuntu 24.04 LTS x64 | Account → API |

## The flow

1. Pick the provider and paste a token (**Use it**). With **Keep it in the system's keychain** on, the token is kept in the credential store as `vps-token:<provider>`, read back the next time, and used to delete the server later.
2. Pick a region, then a size (cheapest first, with its CPUs, memory, disk and monthly price), and a name.
3. The server gets the user's SSH public key for root: `~/.ssh/id_ed25519.pub`, `id_ecdsa.pub` or `id_rsa.pub`, the first there is; **Make one** runs `ssh-keygen -t ed25519` without a passphrase when there is none. A key the provider holds already (same type and data, whatever its comment) is used again rather than added twice.
4. **Create** creates the server, waits until the provider says it runs with a public IPv4 address (polling every 3 seconds), then until its SSH port answers, adds `root@<address>` to the hosts with the provider and the server's ID, and connects: enowX installs its daemon and offers to run as a user of its own, as for any server. Set up… then brings the toolchains and agents.

A host enowX created has **Delete the server at <provider>…** in its ⋯ menu, which asks first, deletes the server with the token from the keychain, and removes the host.

## Tests

`core/vps` is tested against fakes of each API that answer as their documentation shows; the dialog against a fake of Hetzner's and a local port standing for SSH. No real server is created by the tests.
