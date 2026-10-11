# UI redesign: Svelte, keyboard-first, Ukrainian and English

Status: done (step 6 open) · 2026-10-11 · decision in [ADR 0006](../adr/0006-svelte-spa.md)

## Brief

- Subject: a local instrument panel that shows what Claude Code carries into each project, what
  it costs, and what to fix. Audience: developers using Claude Code across many projects and
  accounts. Primary job: **show the next thing worth fixing and let you fix it.**
- Complaints about the MVP: unclear what to do, awkward navigation, raw look, clumsy profiles.
- Direction from the user: like Linear / Raycast (dark, dense, fast, keyboard), uk + en.

## Tokens

Colour: dark by default, a light variant for light system themes. Named after a khipu's
materials: cords were dyed, and colour carried meaning, so colour here is data, not decoration.

| Token | Dark | Role |
|---|---|---|
| night | `#171A23` | background, a blue slate rather than tinted black |
| loom | `#1F2330` | raised surfaces: panels, palette, menus |
| warp | `#2C3142` | lines and dividers |
| cotton | `#E8E4D8` | text, the colour of undyed cotton |
| fleece | `#9A968A` | secondary text |
| indigo | `#7C93F0` | focus, selection, links, primary actions |
| cochineal | `#E06A6A` | findings and problems |
| qolle | `#E2B14A` | cost: cache writes, heavy context |
| chilca | `#74B37A` | applied, signed in, saved |

Type: **Manrope** (variable, has Cyrillic) for everything; numbers use tabular figures.
**JetBrains Mono** only for things that are code: file paths, diffs, commands. Scale 12 / 13 /
14 (base) / 16 / 20 / 28; weights 400, 500, 650. Sentence case, no all-caps labels.

Layout: narrow sidebar (icon + label), a top bar with the page title and the ⌘K trigger, the
content as lists rather than card grids, and a detail panel that slides in from the right when a
row is selected (Linear's master–detail). Left-aligned everywhere.

```
┌────────┬──────────────────────────────────────┬──────────────┐
│ nav    │ title                   period   ⌘K  │              │
│        ├──────────────────────────────────────┤  detail      │
│        │ khipu strip (overview only)          │  panel       │
│        │ list rows ……………………………………………… n │  (on select) │
│profiles│                                      │              │
└────────┴──────────────────────────────────────┴──────────────┘
```

## The one bold element: the khipu

The overview opens with a khipu drawn from real data: a primary cord across the top, one pendant
cord per project. Cord length = cache writes over the period, colour depth = share of the total
(per-project profiles are not known yet), knots = findings. Hovering a cord shows the project; clicking opens it. Everything else stays quiet.

## Screens

- **Overview**: khipu; "Needs attention" (findings across projects, biggest saving first, Apply
  inline); a short row of numbers, each with its change against the previous period.
- **Project**: path and profile; one segmented bar of what is sent at session start (Claude Code
  itself, always-loaded files, on demand); the item list; tabs Context / Advice / Activity.
- **Advisor**: inbox grouped by where the fix goes (your settings, one project); detail panel with
  the explanation, diff, Apply / Undo; history of applied changes.
- **Cache**: daily chart with axis values and tooltips; miss reasons in plain words.
- **Profiles**: list of accounts; "Add profile" and "Sign in" open a side sheet with the steps.
- **Command palette** (⌘K / Ctrl+K): go to a project or page, apply the top finding, switch
  period or language. Keys: `g o/p/c/a/r` go to a page, `j/k` move, `Enter` open, `Esc` close,
  `?` shortcuts.

## Principles

1. The next action comes first; numbers support it, never lead.
2. Every number has context: compared with the previous period or with the project median.
3. Plain words over internal names: rule IDs and API miss reasons get human titles in both
   languages.
4. Everything works from the keyboard; focus is always visible.
5. Spend boldness only on the khipu.

## Review against generic defaults

- Dark + one acid accent is a common default; here the palette is several muted natural dyes,
  and each carries a meaning (cost, problem, done, action).
- Monospace for small data labels is a common tell; numbers stay in Manrope with tabular figures,
  mono is limited to real code.
- A card grid with identical radii is the SaaS default; screens use lists and one panel instead.
- A big-number hero is the dashboard default; the overview leads with the khipu and actions.

## Steps

1. [x] ADR 0006; `web/` with Svelte 5, Vite, Tailwind 4, fonts bundled; Node in Docker; `make web`;
   CI builds the frontend before Go.
2. [x] JSON API in Go for every screen; SPA fallback; same token, Host and Origin checks.
3. [x] Shell: sidebar, top bar, router, i18n (`web/src/locales/{uk,en}.json`), command palette, keys.
4. [x] Screens: overview with khipu, project, advisor, cache, profiles.
5. [x] Remove the html/template pages; screenshots on the demo home; release.
6. [ ] Native window with Wails right after (user's request): the frontend talks to Go only
   through `web/src/lib/api/client.ts`, and Go exposes the API as plain methods with thin HTTP
   handlers, so Wails can bind the same methods without a port or token. Linux needs
   libwebkit2gtk at runtime; cover it in install.sh and the docs.
