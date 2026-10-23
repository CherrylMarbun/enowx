# Preview

The **Browser** tool of the side panel (`⌘⇧P`, or **Preview** on a port's card and in the Ports tool) shows what the selected session's dev server serves, in a web view of this device: WKWebView on macOS, WebView2 on Windows, WebKitGTK on Linux. The plan behind it, and what comes next, is in [plans/preview.md](plans/preview.md).

## How it works

- The tool asks the session's daemon for the ports its processes listen on (`session.ports`, every 2 seconds while it shows) and opens the first, or the one it showed last; several ports give a picker.
- On this machine the page loads `http://localhost:<port>`. On a server, the app forwards the port here first, on the same port when it is free, through the SSH connection (see Ports in [side-panel.md](side-panel.md)): the page renders here, and only HTTP and WebSocket traffic crosses, so hot module reload works as it does locally.
- Each session has one web view, made the first time it shows a page and kept while the session lasts: switching tools or sessions keeps the page, its scroll and its state. Where it was (port and path) is remembered between launches.
- When the port closes, as a dev server restarting does, the tool says so and reloads the page when the port is back.

## The toolbar

Back, forward, reload; the address (the path on the port; typing a full URL of the port works too); a screenshot of the page, typed into the session's prompt as a file for its agent to look at (uploaded to a server's session first); DevTools; and **Open in browser**.

## Hot reload on a server

Nothing to set up for most dev servers: they and their file watchers run in the session, next to the files the agent edits. When the local port can't match the remote one (it was taken here), set the HMR client port (Vite `server.hmr.clientPort`, webpack `client.webSocketURL`). HTTPS dev servers pass through; a self-signed certificate needs the browser. OAuth redirects work, as the origin is `localhost`.

## Desktop apps built with web technology

Tauri (`tauri dev`), Electron (`electron-vite dev`) and Wails (`wails dev`) serve their frontend from a dev server: the Browser previews it with hot reload. The page runs without the native bridge (`window.__TAURI__`, Electron's preload APIs); guard or mock it in development.

## Desktop apps with windows, on a Linux server

**Run with a display** in the New session dialog (offered where the host can: Linux with [Xpra](https://xpra.org); the dialog says what a host lacks) gives the session a virtual display of its own (`core/display`): the daemon starts `xpra start :N --bind-tcp=127.0.0.1:<port> --html=on` and sets `DISPLAY=:N` for the session, so whatever it runs — `tauri dev`, `npm run electron:dev`, `flutter run -d linux`, `cargo run`, a GTK or Qt app — opens its windows there. The display's HTML5 client is a port of the session, `:<port> display`: the Browser shows it, windows as windows, keyboard and pointer going back and the clipboard following. Hot reload is the framework's own, on the server. The display ends with its session, and a session continued after a daemon restart gets a new one.

Install Xpra on the server with its distribution (`sudo apt install xpra`) or from xpra.org. Every frame crosses the network, so latency shows; most servers have no GPU, so 3D is slow; audio is off.

## When the server's SSH doesn't forward ports

Forwards go through the SSH connection (`direct-tcpip`). When the server's SSH refuses (`AllowTcpForwarding no`), they go through the daemon instead (`tunnel.open`): bytes travel in frames of the daemon's own connection, at most 256 KB unacknowledged each way, so a large download can't hold terminals up, and the daemon reaches only ports the session's processes (or its display) listen on. A daemon from before tunnels leaves SSH's reason as the error.

## Without a web view

A Linux machine without WebKitGTK has none: the tool says so and offers to open the page in the browser.
