---
name: ship
description: Commit and push changes the right way (checks, privacy scan, Conventional Commit message, confirmed push). Use whenever you are about to commit or push in this repo, even if the user just says "commit" or "push".
argument-hint: [optional commit message]
---

Current state:
!`git status --short`

Steps; stop and report if any step fails:

1. Run `make fmt`, then `make vet` and `make test`. Do not commit failing code.
2. Review `git diff` (staged and unstaged) and list what will be committed. Never commit
   `bin/`, `.cache/`, `.idea/`, `.claude/settings.local.json`, `CLAUDE.local.md`.
3. Privacy scan of the diff: absolute home paths (`/home/<user>`, `/Users/<user>`), email
   addresses other than the configured `git config user.email`, tokens or keys, real
   transcript content, names of real non-public projects. If anything is found, show it and
   ask before continuing.
4. Write a Conventional Commit message (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`,
   `chore:`), imperative, subject ≤ 72 chars, body explains why. Use $ARGUMENTS if given.
   No AI attribution trailers.
5. If the change touches an architectural decision, check that an ADR exists (`/adr-new`).
6. Show the message and file list, then commit. Push only after the user confirms.
7. After push, remind the user to check the CI run on GitHub.
