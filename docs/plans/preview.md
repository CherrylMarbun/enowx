# Plan: browser preview, hot reload on a server, and desktop apps

Status: planned. Supersedes the earlier unwritten plan "Built-in browser preview with automatic port forwarding", rewritten against what enowX does today.

## Goal

See what a session builds without leaving enowX, wherever the session runs:

1. **Web apps.** A dev server a session starts (Vite, Next.js, Astro, a Go or Rails server) shows in a **Browser** tool of the side panel, rendered on this device by its own web view. On a server, only HTTP and WebSocket traffic crosses the connection, so hot module reload (HMR) works as it does locally. No screen is streamed.
2. **Desktop apps built with web technology** (Tauri, Electron, Wails). Their frontend shows in the Browser tool the same way, with hot reload. Their native side (Tauri commands, Electron's main process) runs in the session, on its host.
3. **Desktop apps that need a window** (a whole Tauri or Electron app, GTK, Qt, Flutter desktop, a game) on a Linux server. The app runs on a virtual display on the server, and its windows show in the Browser tool through an HTML5 remote-display client. This one does stream pixels.

## Non-goals

- Replacing a full browser. **Open in browser** stays one click away.
- Exposing ports to the LAN or the internet. Everything binds to loopback.
- Running macOS, Windows, iOS or Android apps on a Linux server: they can't run there. Those run on a host of their own kind (see "Other platforms").
- A general remote desktop of the server.

## What exists today

| Piece | Where | State |
|---|---|---|
| Which TCP ports each session's processes listen on | `core/procstat` (`Listening`: `lsof` on macOS, `/proc/net/tcp` on Linux), `daemon/ports.go`, `session.ports` | Done |
| Sessions found by their processes | `core/session` runs each program under `/bin/sh` named `enowx-session-<id>`; `daemon/stats.go` maps children by argument | Done (recent macOS hides other processes' environment, so `ENOWX_SESSION` alone didn't work) |
| Ports opened or closed, told to clients | `ports.changed` every 5 s while a client is connected; the first look only learns | Done |
| Offer a new port | `ui/portoffers.go` card: Open, or Forward & open | Done |
| Forward a server's port here | `transport.Forward` (loopback, same port when free) over SSH `direct-tcpip` (`client.DialHost`, `transport.SSHClient.DialTCP`) | Done |
| Ports tool | `ui/portview` in the side panel | Done |
| Side panel with tools and tabs | `ui/sidepanel.go` | Done: the Browser goes there as a tool |

## Design

### 1. The Browser tool (`ui/browserview`, `ui/browser.go`)

A tool of the side panel, after Ports, for the selected session.

- **Web view:** `mygo.Window.NewWebView(mygo.WebViewOptions{URL: …, Page: mygo.PageOptions{DevTools: mygo.DevToolsEnabled}})`, placed with `ui.WebView(c, view)`. Check `mygo.WebViewAvailable()` first; without one (Linux without WebKitGTK), the tool says so and offers **Open in browser**.
- **One web view per session**, made the first time the tool shows it and kept while the session exists, so switching tools or sessions doesn't reload the page. Destroyed with the session.
- **Toolbar:** back, forward, reload (`Page.GoBack`, `GoForward`, `Reload`), the address (path editable; the origin is the session's port), a port picker when the session listens on several, **Open in browser**, **DevTools** (`Page.ToggleDevTools`), and a screenshot to attach to the agent (`WebView.CapturePage`, sent like a dropped file: uploaded to the server's session when it is remote).
- **States:** "Waiting for a dev server…" (no port yet), loading, "The dev server stopped" (the port closed; it reloads by itself when the port comes back, as dev servers restart often), "Can't reach :5173" with the error.
- **Remembered per session** (`ui/state.go`): the last port and path.
- **Opening it:** the port offer card gets **Preview** next to Open; the Ports tool's rows too; `⌘⇧P` toggles the Browser tool.

### 2. Forwarding for the preview

- A server's port is forwarded the first time something previews or opens it (as the Ports tool does now), on the same local port when free: HMR clients that connect to the page's port then work unchanged.
- A forward lives while its session does, through dev-server restarts: the local listener stays, and connections made while the port is down fail until it is back. It stops when its session ends, on **Stop**, or with the connection.
- **Transport.** Today: SSH `direct-tcpip`. Some servers forbid it (`AllowTcpForwarding no`), and the daemon can't then refuse ports that aren't a session's. Planned as a later phase, a tunnel in the daemon protocol:
  - `tunnel.open {Session, Port}` → `{Stream}`. The daemon dials `127.0.0.1:<port>` (or `[::1]`) only if one of that session's processes listens there.
  - Frames `TypeTunnelData`, `TypeTunnelClose` on the stream's channel, and `TypeTunnelAck` credits (a 256 KiB window per channel), so a large download can't starve terminal frames; the connection's writer sends terminal frames first.
  - New frame types need `proto.Version` + 1, with the app falling back to SSH `direct-tcpip` for an older daemon.
  - The app uses the tunnel when SSH forwarding is refused, and for transports without SSH.

### 3. Hot reload on a server

Nothing else to build for web apps: the dev server runs in the session on the server, files change there (the agent edits them there), and HMR's WebSocket goes through the forward. What needs care, documented in `docs/preview.md`:

- **Same port.** Keep the local port equal to the remote one; else set the HMR client port (Vite `server.hmr.clientPort`, webpack `client.webSocketURL`).
- **Host checks.** The page loads from `localhost`, which dev servers accept by default.
- **File watching.** Watchers run on the server, next to the files: no sync, no polling across the network.
- **HTTPS dev servers.** TLS passes through the TCP forward; a self-signed certificate needs the web view's prompt, or Open in browser.
- **OAuth redirects** work, as the origin is `localhost`.

### 4. Desktop apps built with web technology

- **Tauri** (`tauri dev`), **Electron** (`electron-vite dev`, `electron-forge start`), **Wails** (`wails dev`) start a frontend dev server; the Browser previews it with HMR, as any web app.
- The page then runs without the native bridge (`window.__TAURI__`, `invoke`, Electron's preload APIs). Apps guard or mock it in dev; enowX documents the pattern. Wails serves a browser-mode bridge on its dev server, so it works more fully.
- The native side runs in the session; its logs show in the terminal.
- On macOS, the web view is WKWebView, the engine Tauri uses there: what the preview shows is close to the app.

### 5. Desktop apps with windows on a Linux server

For a whole app (its native window, menus, dialogs), or one not built with web technology.

- **Display per session.** The New session dialog gets **Run with a display** (off by default). The daemon then starts an [Xpra](https://xpra.org) server for the session on a free display, listening on a loopback port with its HTML5 client (`xpra start :N --bind-tcp=127.0.0.1:<port> --html=on --daemon=no`), and sets `DISPLAY=:N` in the session's environment, so whatever the session runs (`tauri dev`, `npm run electron:dev`, `flutter run -d linux`, `cargo run`) opens its windows there. It ends with the session.
- **Seen in the Browser.** The display's port is a session port like any other: the Browser tool loads Xpra's HTML5 client through the forward. Windows show as windows (Xpra forwards each, not a whole desktop), keyboard and pointer go back, the clipboard follows. A **Display** entry sits in the port picker.
- **Hot reload** is the framework's own: the app restarts or reloads on the server as files change, and its window shows again in the client.
- **Why Xpra:** per-window, an HTML5 client that needs nothing installed here, compression suited to the internet, and it runs as the user without root. Alternatives considered: Xvfb + x11vnc + noVNC (a whole screen, more bandwidth, more parts), Wayland headless + wayvnc (fine for Wayland-only apps, less mature), Broadway (GTK only).
- **Installing it.** `xpra` comes from the distribution or Xpra's repository; the New session dialog offers to install it as it does agents (`docs/agents.md`), and the VPS setup of Phase 4 adds it to its checklist.
- **Limits, said in the UI:** latency of the network on every frame; no GPU on most servers (software rendering, fine for UI, slow for 3D); audio off.

### 6. Other platforms

- **macOS and iOS apps** need a Mac. enowX already runs its daemon on a Mac as a host; sessions there build and run the app, but seeing its window from another device needs screen sharing, which is out of scope. Building on this machine stays the way.
- **Windows apps** need a Windows host, the same way.
- **Android:** the emulator on a server needs KVM; screen mirroring (scrcpy) is a later topic.

## Phases

1. **Browser tool, this machine.** `ui/browserview`: web view, toolbar, states, per-session web views, DevTools, Preview on the port card and in the Ports tool, `⌘⇧P`. Tests: headless UI tests of the toolbar and states with a fake page; a manual run with `npm create vite` + `npm run dev`.
2. **Browser tool, servers.** Preview forwards on first use; forwards end with their session; screenshot to agent (uploaded for remote sessions). Tests: end-to-end through the daemon with an HTTP and a WebSocket server in a session, through `transport.Forward`.
3. **Desktop apps with a display.** Run with a display in the New session dialog; the daemon's Xpra lifecycle (`core/display`, Linux only, build-tagged); the Display entry in the Browser; offering to install Xpra. Tests: the daemon starts and stops the display with the session (skipped without `xpra`); a manual run of a GTK demo and `tauri dev` on an Ubuntu server.
4. **Daemon tunnel.** `tunnel.open`, the frame types, credits, `proto.Version` bump with the SSH fallback. Tests: bytes in equal bytes out both ways, a WebSocket echo, a 50 MB download while a session streams output (terminal frames stay responsive), close and reopen, a port that isn't the session's refused.
5. **Polish.** "Always preview this port", the Browser as a session view next to Terminal and Chat for wide windows, a responsive-size picker (phone, tablet, desktop widths).

## Security

- Forwards and the daemon tunnel reach loopback ports of the session's own processes only; local listeners bind to `127.0.0.1`.
- Nothing about requests is logged: no bodies, headers or cookies.
- Each host gets a web view data store of its own, kept on this device; deleting a host clears it.
- Xpra listens on loopback only, reached through the forward; no password travels, as the connection is already the user's SSH.

## Open questions

- Is `mygo.WebViewAvailable()` true on the Linux and Windows builds enowX ships, and what does WebKitGTK need at run time?
- Should the Browser also be a session view (next to Terminal and Chat) for those who preview all day, or is the side panel enough? Phase 5 assumes both.
- Does Xpra's HTML5 client behave in WKWebView and WebView2 as in Chrome (keyboard shortcuts, clipboard)? To check in phase 3.
- Is a display per session right, or one per project, shared by its sessions? Per session to start: it ends cleanly with the session.
