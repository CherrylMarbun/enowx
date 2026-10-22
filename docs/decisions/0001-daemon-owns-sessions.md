# 1. The daemon owns sessions and mirrors screens to clients

Status: accepted (phase 0 spike)

## Context

enowX must run agents locally and on a VPS with the same code, keep them running while the app is closed, and restore exactly what each terminal shows when the app reconnects. Agent TUIs redraw with relative cursor movement, so replaying raw output into a fresh terminal can show something different from what the program drew.

## Decision

- Every session lives in `enowxd`, locally (started by the app) or on a server (reached over SSH through `enowxd bridge`).
- The daemon emulates each session's screen in a headless MyGo terminal. Each attached client gets a `Mirror`, whose frames bring the client's terminal to the daemon's screen cell for cell.
- The client's terminal uses the stream as its `Conn` with `ResizeByConn`. Size changes travel in-band (`TypeResize`) ahead of the frames drawn at the new size.
- `core/session` may import MyGo's `plugins/terminal` (never `ui`). All other non-UI packages stay MyGo-free.

## Consequences

- Reattach is exact and cheap; multiple clients can watch one session.
- The daemon depends on `libghostty-vt` (loaded at run time, no cgo).
- The daemon polls mirrors for changes; polling is adaptive to keep idle CPU low (see `daemon/stream.go`).
