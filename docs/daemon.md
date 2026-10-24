# The daemon's lifetime

Each host runs one daemon (`enowxd`, or `enowx daemon` on a desktop), which owns its sessions. The app connects to it and can come and go.

## On this device

Quitting the app leaves the daemon running: agents keep working, automations run, and the next launch finds every session as it was. To have them stop:

- **Quit and stop all agents**: in the app menu on macOS (⌥⌘Q) and in the command palette everywhere.
- **Settings → Stop this device's agents when enowX quits**, off by default.
- **Stop all agents** in the command palette stops the connected host's agents without quitting.

Stopping an agent ends its program the way hibernation does (`daemon.stopAgents`): the session stays, showing what it showed, and its agent comes back on its conversation when the session is shown. Plain terminals keep running.

## On a server

Connecting installs the daemon at `~/.enowx/bin/enowxd` and starts it through `enowxd bridge`. Where the server runs systemd, it starts as the user's service (`~/.config/systemd/user/enowxd.service`, `core/userservice`), and the app turns on lingering for that user while preparing the server (`hostconn/linger.sh`, as root, or with sudo or doas without a password). The daemon then starts again after a reboot without anyone connecting, its sessions come back, and automations keep running.

Where there is no systemd (Alpine, containers) or lingering couldn't be turned on, the daemon runs as a process as before: it lives until the server reboots, and the next connection starts it. Why a service didn't start is written to `~/.enowx/enowxd.log`.

`scripts/test-service.sh` checks it in an Ubuntu container booted with systemd.
