# 0007. Native window with Wails in a separate binary

Status: Accepted · 2026-10-11

## Context
The UI opens as a browser app window (`--app`) or a tab, served on a loopback port with a token.
A native window gives quipu its own window and icon, needs no port or token, and does not depend
on which browser is installed. Wails v2 (stable; v3 is still beta) wraps the system webview:
WebKitGTK on Linux, WKWebView on macOS. On Linux it links libwebkit2gtk-4.1 and GTK 3 through CGO,
so a binary that contains it does not start at all where those libraries are missing (servers,
minimal containers).

## Decision
The window is a second binary, `quipu-app`, shipped in the same archive. It runs the same
`ui.Service` in-process and gives Wails the same HTTP routes as its asset handler, without the
token and Host checks: requests never leave the process. `quipu ui` starts `quipu-app` when it is
next to `quipu` and loads, and otherwise falls back to the browser as before. `quipu` itself
keeps no GUI dependencies.

## Rejected alternatives
- Wails inside `quipu`: the CLI and daemon would need GTK and WebKit everywhere.
- Wails bindings (generated JS for Go methods): a second transport in the frontend; the HTTP
  routes through the asset handler need no frontend changes.
- Wails v3: still beta.

## Consequences
Linux builds need libwebkit2gtk-4.1-dev and libgtk-3-dev (a build image locally, apt in CI); users
need the runtime library, which desktop distributions usually have. The app and the daemon may both
collect into the same store; writes are atomic renames, so this is safe. macOS gets a plain
binary for now, not an `.app` bundle.
