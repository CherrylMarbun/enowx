# The daemon's lifetime

Each host runs one daemon (`enowxd`, or `enowx daemon` on a desktop), which owns its sessions. The app connects to it and can come and go.

## On this device

Quitting the app leaves the daemon running: agents keep working, automations run, and the next launch finds every session as it was. To have them stop:

- **Quit and stop all agents**: in the app menu on macOS (⌥⌘Q) and in the command palette everywhere.
- **Settings → Stop this device's agents when enowX quits**, off by default.
- **Stop all agents** in the command palette stops the connected host's agents without quitting.

Stopping an agent ends its program the way hibernation does (`daemon.stopAgents`): the session stays, showing what it showed, and its agent comes back on its conversation when the session is shown. Plain terminals keep running.
