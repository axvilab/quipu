---
paths:
  - "web/**"
---

# Frontend (web/)

Svelte 5 (runes) + Vite + Tailwind 4, built into `internal/ui/dist` and embedded in the Go binary
(ADR 0006). Design decisions live in `docs/plans/2026-10-11-ui-redesign.md`; follow them.

- Every user-visible string comes from `web/src/locales/{uk,en}.json` through `t('key')`. Add the
  key to both files in the same change. No hardcoded text in components.
- Use the colour tokens (night, loom, warp, cotton, fleece, indigo, cochineal, qolle, chilca), not
  raw hex or Tailwind palette colours. Colour carries meaning: qolle = cost, cochineal = problem,
  chilca = done, indigo = action/focus.
- Manrope for UI with tabular numbers; JetBrains Mono only for paths, diffs and commands.
- Sentence case. No all-caps labels, no middle-dot meta strings, no arrows appended to buttons.
- Lists and a detail panel, not card grids. One bold element per screen at most.
- Keyboard first: every action reachable from the keyboard and the command palette; visible focus;
  respect `prefers-reduced-motion`.
- The Go server owns logic and safety. The frontend only calls `/api/*` (cookie auth; mutating
  requests are POST and rely on the Origin check). Never compute advice or edit files in JS.
- No new npm dependency without a reason in the commit message; prefer a few lines of code.
