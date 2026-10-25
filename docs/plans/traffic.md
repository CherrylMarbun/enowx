# Plan: HTTP traffic: see, mock and change what apps send

Status: planned, nothing built yet.

## Goal

See every HTTP request what a session builds sends and receives, wherever it runs, change it on the way, and answer it with mocks, as one part of enowX that works with the rest of it:

1. **Capture** HTTP/1.1, HTTP/2, WebSocket, server-sent events and gRPC, over TLS too, from the apps a session runs, the Browser tool's pages, desktop apps on a server's display, and Android devices.
2. **Inspect** each exchange: headers, bodies decoded and shown by their kind, timings, TLS, the session or device it came from.
3. **Mock and rewrite** with rules: answer from a mock, forward elsewhere, delay, fail, transform requests and responses, pause them at a breakpoint to edit by hand.
4. **Send** requests of one's own, from scratch or from one captured, and export them as code.
5. **Agents use it**: read the traffic, add mocks, get a failing request handed to them, from the command line or the app.

Everything is enowX's own code, written from the protocols and from how such tools behave, not from another tool's sources (see "Clean room").

## Non-goals

- A system-wide proxy for the whole machine by default: interception is per session, per device, per view, and opt-in.
- Breaking certificate pinning of other people's apps. Apps being debugged are the user's own, built in debug, where trusting a debug CA is a setting of the app.
- A hosted service or sharing traffic online. Captures stay on the host, export is a file.
- Non-HTTP protocols beyond passing them through (raw TCP, MQTT, databases): shown as tunnels with their sizes, not decoded.

## Where it fits

```
                    host A (a server)                              this laptop
 ┌──────────────────────────────────────────────┐        ┌──────────────────────────────┐
 │ session: app, tests, dev server ──proxy env──┐│        │ Traffic tool (side panel)     │
 │ desktop app on its display ──────proxy env──┤│ events │  requests · rules · compose   │
 │                                              ▼│ ─────▶ │ Browser tool's web view ─┐    │
 │                              interceptor (daemon)◀───── │ Android emulator ────────┤    │
 │                               CA · rules · capture    │ │        (adb reverse) ────┘    │
 └──────────────────────────────────────────────┘        └──────────────────────────────┘
```

- **One interceptor per host**, in its daemon (`core/intercept`), listening on loopback only. Traffic of that host's sessions goes to it directly; traffic from elsewhere (the laptop's web view, a device on the laptop) reaches it through enowX's connection, as ports and ADB already do.
- **The app shows every host's traffic** it is connected to, in the **Traffic** tool of the side panel, filtered to the selected session by default.
- **Rules live with the project** (`.enowx/traffic.json`, committed if the user wants), so the server and the laptop use the same mocks, Sync and Copy carry them.

## The engine (`core/intercept`, no MyGo)

### Listening and protocols
- One loopback port per host, speaking **HTTP proxy** (absolute URLs and `CONNECT`) and **SOCKS5** on the same port, told apart by the first bytes.
- Inside a `CONNECT` or SOCKS tunnel: TLS (by its first byte) is terminated, plain HTTP is read as is, anything else is passed through and recorded as a tunnel.
- **TLS:** a certificate authority per host (`~/.enowx-app/intercept/ca.pem`, its key `0600`, never sent anywhere), leaf certificates made per server name and cached, ALPN mirrored: `h2` when the client offers it and the upstream accepts it, else HTTP/1.1. Upstream TLS checked as normal, with per-host exceptions (self-signed dev servers) and client certificates per host.
- **HTTP/2** (`golang.org/x/net/http2`, server and client), **WebSocket** (the upgrade, then frames recorded both ways, with their opcodes), **SSE** (events split as they come), **gRPC** (HTTP/2 with length-prefixed messages; decoded as protobuf without a schema, or with the project's `.proto` files when found).
- **Upstream:** directly, or through the user's own upstream proxy; DNS overrides per name (point `api.example.com` at a local server).

### Who sent it
Each source gets credentials for the proxy (`http://<source>:<token>@127.0.0.1:<port>`): a session, a tab, a device, a web view. The interceptor refuses requests without valid ones, so other users of a shared server can't use it, and every exchange carries its source.

### Capture
- An exchange: method, URL, request and response headers and bodies, status, HTTP version, timings (connect, TLS, sent, first byte, done), sizes on the wire and decoded, TLS version, cipher and server name, the rule that matched, the source, errors (refused, reset, timed out, certificate rejected).
- Bodies are decoded for display (gzip, deflate, brotli, zstd) and kept as received.
- **Kept in memory** with limits (the last 2,000 exchanges, 256 MB of bodies, settings), spilling large bodies to `~/.enowx-app/intercept/bodies/` and deleted with the capture; **Clear**, and cleared when the daemon stops unless kept.
- **Secrets:** `Authorization`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`, and headers or fields named like keys are masked in the list and whenever an exchange goes to an agent, shown only on **Reveal**.

### Rules
Ordered, first match wins, each enabled or not, with a count of hits:

- **Match on:** method, host, path (exact, prefix, glob, regular expression), query parameters, headers, body (contains, regular expression, a JSON field's value), the source (a session, a device), HTTP or WebSocket.
- **Then:**
  - pass through (the default);
  - **mock**: a status, headers and a body (typed, or a file of the project), optionally only the first N times;
  - **forward** to another origin (keep the path), rewriting the Host header or not;
  - **transform** the request or the response: set or remove headers, replace in the body, patch JSON fields, change the status;
  - **delay**, **throttle** (bytes per second), **fail**: close, reset, time out, or answer 5xx;
  - **breakpoint** on the request, the response or both: the exchange waits, its fields editable, until **Resume**, **Drop** or a time limit;
  - WebSocket: mock messages, echo, close.
- Rules are data (`traffic.json`), checked when saved; a rule that can't apply says why.

### Sending
- A request built by hand (method, URL, headers, body as text, JSON, form or file), or **Edit and send** from a captured one, sent from the host it is for, through the rules or around them; the answer shows like any exchange, and a history is kept per project.
- **Copy as** curl, fetch, Python requests, Go `net/http`, written by enowX.
- **Export** a selection or the capture as HAR, **import** HAR to view or to make mocks from.

## Interception points: how each part of enowX is caught

| Source | How its traffic reaches the interceptor | Trust of the CA |
|---|---|---|
| **A session's commands** (the app, tests, scripts, a dev server's outgoing calls) | Environment for the tabs where it is on: `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY` (with the source's credentials), `NO_PROXY` for loopback; `NODE_USE_ENV_PROXY=1` (Node 24+) | `SSL_CERT_FILE` and `SSL_CERT_DIR` (a bundle of the host's roots plus the CA, since some tools replace rather than add), `NODE_EXTRA_CA_CERTS`, `REQUESTS_CA_BUNDLE`, `CURL_CA_BUNDLE`, `GIT_SSL_CAINFO`, `PIP_CERT`, `AWS_CA_BUNDLE`, `DENO_CERT`, `JAVA_TOOL_OPTIONS` (proxy properties and a trust store), `DART_VM_OPTIONS` where it applies |
| **The agent itself** | Off by default: the agent's tab doesn't get the environment, so its own API calls (with its keys) stay out. A switch per session puts it in, for debugging an agent. | as above |
| **Desktop apps on a server's display** (`core/display`) | They run in the session with the same environment; Chromium-based apps (Electron) get `--proxy-server` and the CA through the environment Chromium reads (`NSS` database for the display) | the bundle, and an NSS database made for the display |
| **The Browser tool's web view** | Two parts. **Dev server traffic** (the page and its HMR) already goes through enowX's forward: the forward records it as it carries it, no proxy needed. **Other origins** the page calls (an API on the internet): the web view's proxy setting, which MyGo doesn't offer yet (WKWebView `proxyConfigurations` on macOS 14+, WebView2's `--proxy-server`, WebKitGTK's network proxy settings) — added to MyGo, then pointed at the interceptor through a forward. | the web view's own trust: a per-web-view exception for the CA where the platform allows it, else the user trusts the CA for this device (asked, with what it means) |
| **Android devices** (`docs/plans/mobile.md`) | The device's proxy set to `127.0.0.1:<port>` (`settings put global http_proxy`), with `adb reverse` taking that port to the device's host and enowX's forwarding taking it on to the interceptor of the session's host. Apps that ignore the system proxy (Flutter's Dart `HttpClient`) are caught on emulators by redirecting their traffic (`iptables` on a root emulator) or by the app's debug code reading the proxy (a two-line `HttpOverrides`, which the agent can add). | Emulators: the CA added to the system store (`adb root`, the system store, or the APEX store on Android 14+). Phones: the CA installed as a user CA, and the app's debug build trusting user CAs (`network_security_config` `debug-overrides`), which enowX offers to add to the project |
| **Docker containers a session starts** (later) | Proxy environment and the CA mounted into containers started from the session | the bundle mounted |

Interception is **turned on per source**, never globally: a session's **Intercept** switch (its run tabs, new ones from then on), the Browser tool's switch, a device's switch in the Android tool.

## The Traffic tool (side panel)

A tool of the side panel, after Browser, for the selected session; it opens wide (the side panel can take most of the window) or as the session's view.

- **Requests:** a list (method, host and path, status, type, size, time, source), live, with the session's by default or every source of the host, filters (text, method, status class, type, has a rule matched, errors), pinning, and **Clear**. Selecting one shows it below or beside: request and response, each with headers and a body viewer:
  - JSON (tree and text, searchable), XML and HTML (formatted), images, text, hex, URL-encoded and multipart forms, GraphQL (the operation, its variables), JSON-RPC, SSE events, WebSocket messages in order, gRPC messages (protobuf, decoded), with **Copy**, **Save**, **Reveal secrets**;
  - timings as a bar, TLS details, the rule that matched;
  - actions: **Mock this**, **Breakpoint on this**, **Edit and send**, **Copy as…**, **Send to agent**.
- **Paused:** exchanges waiting at a breakpoint, a count on the tool's tab; each editable (status, headers, body) with **Resume** and **Drop**; the session shows "needs you" while one waits.
- **Rules:** the ordered list, each in a short sentence ("GET api.example.com/users → mock 200, 3 hits"), edited in a form, reordered by dragging, on and off; **Add from a request** fills the match from it.
- **Compose:** the request editor and its history.
- **Setup notes:** when a source can't be caught (an app ignoring the proxy, the CA not trusted), a line says why and what to do, from the errors the interceptor sees (a TLS handshake refused because of the certificate).

## Working with the rest of enowX

| Feature | What it gains |
|---|---|
| **Agents** | `enowx-app traffic list|show|watch|mock|rules|send|clear` through the daemon, for scripts and agents: read the session's traffic (secrets masked), add a mock to test an error path, wait for a request (`watch --path /api/login --status 5xx`). The agent guide gets a short section. **Send to agent** from an exchange puts it (masked) in the session's prompt, as screenshots and picked elements are sent today. |
| **Chat view** | An exchange sent to the agent shows as an attachment in the conversation. |
| **Browser tool** | Its page's requests show in Traffic; from an exchange, **Open in Browser**. |
| **Android tool** | **Intercept** per device; the device's traffic in Traffic, as a source. |
| **Ports** | A port's row offers **Intercept** for traffic to it (a reverse-proxy rule in front of the session's own server, to see what reaches it). |
| **Run menu / Run app** | Run commands start with interception when the session's switch is on. |
| **Checks** | Tests run with the project's rules (mocks for external APIs), when the project says so. |
| **Sync and Copy** | Rules travel with the project; captures don't. |
| **Fan-out and Compare** | Each fanned-out session is its own source: compare which requests each version makes. |
| **Notifications and status** | A breakpoint waiting marks the session "needs you"; an app's 5xx or connection errors can raise a notification (a setting). |
| **Resource usage** | The capture's memory shows with the daemon's. |
| **Settings** | The CA (show, export, regenerate), limits, masked headers, the upstream proxy, per-host TLS exceptions and client certificates. |

## Protocol additions (`daemon/rpc_traffic.go`)

- `traffic.status`: the port, the CA's fingerprint, limits, sources on.
- `traffic.source.on` / `off`: a source's credentials and environment (for a session, a device, a view).
- `traffic.list` (summaries, paged), event `traffic.exchange` (a summary as it starts, updates as it finishes), `traffic.get` (one exchange; bodies in parts, as transfers are read), `traffic.clear`.
- `traffic.rules.get` / `set` (the project's), `traffic.paused`, `traffic.resume` (with edits) / `drop`.
- `traffic.send` (a composed request, answered as an exchange), `traffic.export` / `import` (HAR).
- `traffic.ca` (the certificate, to install where it must be trusted).

Summaries are small (no bodies) so the list stays live over a far connection; bodies come when an exchange is opened. An older daemon answers them as unknown; the tool says the host needs updating.

## Phases

1. **Engine and sessions.** The proxy (HTTP proxy and SOCKS5, TLS with the host's CA, HTTP/1.1 and HTTP/2), sources and their credentials, the capture with limits and masking, a session's **Intercept** switch with its environment and trust bundle, the Traffic tool's list and details (headers, JSON, text, images, hex), `enowx-app traffic list|show|watch`. Done when Node, Python, Go and curl in a server's session are captured over HTTPS and shown on the laptop, attributed to their session, and the agent's own calls are not.
2. **Rules.** Mocks, forward, transform, delay, fail, breakpoints with editing; the rules in the project; the Paused list and "needs you"; `enowx-app traffic mock|rules`. Done when a mock answers a session's app, a breakpoint holds a response until edited, and an end-to-end test through the daemon checks each action.
3. **Views and devices.** The forward's recording for the Browser tool, the web view's proxy once MyGo has it, desktop apps on a display (environment, NSS), Android devices (proxy, `adb reverse`, the CA on emulators, debug trust on phones). Done when a page in the Browser, an Electron app on a server's display and a Flutter app on an emulator all show their HTTPS calls.
4. **Protocols and sending.** WebSocket, SSE, gRPC viewers; Compose with history; Copy as; HAR export and import; GraphQL and form viewers; search across bodies.
5. **Integration.** Send to agent and its chat attachment, Ports' reverse-proxy interception, Checks with mocks, notifications on errors, Settings' TLS exceptions and client certificates, Docker containers.

## Risks

- **Apps that ignore the proxy or pin certificates** (Dart, some SDKs): caught on root emulators by redirection, elsewhere they need a debug hook in the app; the tool says which and offers the change.
- **Trusting a CA** is a security decision: the CA is per host, its key never leaves it, trust is added per source (environment, emulator), and trusting it on this device is asked explicitly with what it means.
- **Secrets in captures:** masked by default, never sent to an agent unmasked, captures kept in memory and cleared on stop; a warning when a capture is exported.
- **HTTP/2 and gRPC edge cases** (trailers, flow control, server push off): tested against real servers and clients in the end-to-end tests.
- **Memory:** large downloads and streams through the proxy: bodies past a size are recorded only as their size and type, a setting.
- **Web view proxies need MyGo:** until it offers them, the Browser tool shows dev-server traffic only.

## Clean room

- Written from the HTTP, TLS, HTTP/2, WebSocket and gRPC specifications, Go's standard library and `golang.org/x/net`, and how such tools behave from the outside (their documentation and use), never from another interception tool's source code. In particular, no AGPL-licensed source is read or translated.
- No other product's name, texts, icons or assets.
- Libraries used keep their licenses, listed in `THIRD_PARTY_NOTICES` when they ask for it (brotli and zstd decoders, for instance).
