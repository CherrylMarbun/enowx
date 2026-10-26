# Floating control

A round panel over whatever is on screen: six actions around a button and a menu at its front for the sessions of this machine or a server, one shortcut away (`ui/floating.go`). It is a window of its own, a MyGo `Panel`, frameless and transparent, kept above other apps' windows. It is off by default; **Settings → Floating control** turns it on, and the same section lists its shortcut with the keyboard shortcuts.

- **The shortcut:** `⌘⇧Space`, Command+Shift+Space on macOS and Control+Shift+Space on Windows and Linux (not Command+F, which every app needs for Find). It works while enowX runs, whatever app is in front: the panel blooms at the pointer, and press it again and the panel folds away; the key only shows and hides it, the feature staying on.
- **The panel:** seven separated segments with larger icons and persistent action labels around a spacious center button. Chevrons mark the Sessions and Panels submenus. Hovering adds the theme's accent tint and an inner arc; an open submenu keeps its arc lit. The center shows the hovered action's name. The panel springs open and folds away.
- **Dismissing:** Escape and the center's Back button return one level at a time: sessions → projects → servers (when offered) → actions. From the actions, Escape folds an open submenu first and then the panel; the center folds the panel directly.
- **Submenus:** Sessions and Panels ease open in 120 ms and accept clicks while opening, so a quick second click is not lost waiting for the animation.
- **Appearance:** follow the app's accent or pick a floating-only color from the swatches or a custom hex value. Tint menu segments adds a subtle colored surface; Compact size shrinks both the menu and its window; Reduce animation opens every level directly without spinning or bouncing. These preferences are remembered without changing the app or terminal theme.
- **What it can't do:** an action that needs a project or a connection the app does not have is dimmed and does nothing. Sources and projects with no sessions remain selectable and show an empty state.

## The six actions

- **New session:** the New session dialog, as `⌘N` opens it.
- **Open folder:** this device's folders, the one picked opened as a project, as `⌘O` does.
- **Panels:** the panel switches in one, fanning out three buttons: **Sessions**, the list of projects and sessions; **Side panel**, the tools about the selected session, as `⌘⇧D` does; and **Split**, the project's sessions side by side (`⌘⇧L`), and back to one.
- **Command palette:** every action, session and host, as `⌘K` shows it.
- **Settings:** the rail's Settings.
- **Appearance:** the themes and accent colors.

## Sessions

At the front of the disc, above the top action, a two-option control picks where the ring's sessions come from:

- **This PC:** choose a project on this machine, then one of its sessions.
- **VPS:** when several servers are configured, choose a server first; then choose its project and session. A single server goes straight to its projects.

Server and project choices have icons and visible names. Next and Previous pages keep every choice reachable when there are many. Registered projects with no sessions remain selectable and show an empty state. When project metadata is unavailable, the menu goes straight to the source's sessions.

Picking a project turns the ring into that project's sessions on the selected host: the agent's icon in each segment, its status dot beside it, and the session's name in the hub on hover. A waiting session comes first, then a working one, then the rest. Six show; the hub's tooltip says how many more there are. Clicking a session brings the main window forward on it and folds the panel.

The picker is a choice for one showing: a fresh summon opens on the actions with nothing picked.

The ring uses the same agent logos as the rest of the app, including OMP, Antigravity, Cline and Hermes. Direct agent commands are also recognized for shell and custom sessions, including Windows executable paths. Agents without a dedicated logo use the bot icon; ordinary shells keep the terminal icon.

A shell tab inside an agent session uses the parent session's agent icon and, when unnamed, its name. A tab running another agent keeps that agent's icon. Picking either still opens the original tab, including when the parent's first program has stopped.

## Statuses

The labels are the session list's, from `ui/session.go` (`statusOf`):

| Status | What it means |
| --- | --- |
| `hibernated` | Put away to free memory; it comes back on its conversation. |
| `stopped` | enowX restarted and it did not come back. |
| `done` | The program ended with code 0, or the agent finished its task. |
| `exited N` | The program ended with a non-zero code, N. |
| `queued` | Waiting for fewer agents to work on this host. |
| `needs input` | The agent is waiting for an answer. |
| `working` | The agent is on a task. |
| `idle` | Nothing is happening. |

The colours follow the status: needs input takes the warning colour, working the working colour, done the done colour, idle, queued and hibernated the idle colour, and an exit with a non-zero code the error colour. A hollow dot means nothing is happening: idle, queued, hibernated, and a session the agent has finished with.

## Nothing of its own

The panel drives actions the app already has, and lists the sessions the app already has, so it works the same for a session on this machine and one on a server: a host's daemon needs nothing new, and nothing is sent anywhere.

## The Sessions panel

**Settings → App → Hide the Sessions panel** keeps the list of projects and sessions folded away so the session gets the room. The command palette's **Show/Hide the Sessions panel** folds and unfolds it for the moment, and so does its shortcut, listed in that section with the others. The switch is remembered between launches, in the app's state file, so a panel folded away stays folded when enowX starts again.
