---
name: release
description: Cut a quipu release: checks, choose the version, annotated tag, push, watch the Release workflow, verify the assets.
disable-model-invocation: true
argument-hint: [version, e.g. v0.1.0; omit to get a proposal]
---

Current state:
!`git status --short --branch`
!`git describe --tags --abbrev=0 2>/dev/null || echo "no tags yet"`

Releases are built by `.github/workflows/release.yml` on a `v*` tag (see ADR 0005). Steps; stop
and report if any fails:

1. Preconditions: on `main`, working tree clean, `git fetch` shows main equal to `origin/main`
   (everything pushed through `/ship`). The CI run for HEAD on GitHub is green:
   `curl -s "https://api.github.com/repos/axvilab/quipu/actions/runs?branch=main&per_page=1"`.
2. `make fmt` (no changes expected), `make vet`, `make test`.
3. Changes since the last tag: `git log <last-tag>..HEAD --oneline`. Nothing to release → stop.
4. Version (semver, `v` prefix). Use $ARGUMENTS if given. Otherwise propose one from the commit
   types: before 1.0, a breaking change or a notable new feature bumps the minor
   (v0.1.0 → v0.2.0), only fixes and small features bump the patch (v0.1.0 → v0.1.1).
   A tag with a hyphen (v0.2.0-rc.1) becomes a pre-release, which `install.sh` (latest) skips.
   The tag must not exist yet locally or on origin.
5. Show the version and a short summary of user-visible changes grouped as Features / Fixes,
   then ask the user to confirm.
6. After confirmation: `git tag -a <version> -m "quipu <version>: <one-line summary>"`, then
   `git push origin <version>` (the push needs the user's approval).
7. Watch the "Release" workflow until it completes (poll the Actions API every ~30 s; it takes
   about 5 minutes). On failure, show the failed job's name and log URL and stop; do not delete
   or move the tag without asking.
8. Verify `https://api.github.com/repos/axvilab/quipu/releases/tags/<version>` has
   `quipu_{linux,darwin}_{amd64,arm64}.tar.gz` and `checksums.txt`.
9. Offer to update the local install: `curl -fsSL https://raw.githubusercontent.com/axvilab/quipu/main/install.sh | sh`
   then `systemctl --user restart quipu` (Linux) and `quipu version`.
