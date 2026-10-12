# 0008. Check for and install updates from GitHub Releases

Status: Accepted · 2026-10-12

## Context
quipu ships often, and users only learned about a release from the repository. Updating meant
copying the install command into a terminal and restarting the service. DESIGN.md allowed no
network access except git operations the user starts.

## Decision
quipu asks GitHub for the newest release (the redirect of `/releases/latest`, not the API) only
when the user clicks the check next to the version, or once a day if they turn that on in
Settings; it is off by default. Installing downloads the same archive `install.sh` uses, checks it
against the release's `checksums.txt`, makes sure the new binary starts, replaces quipu (and
`quipu-app`, `Quipu.app`) in place and re-executes the daemon with the same PID, so systemd and
launchd keep tracking it. A dev build never updates; a read-only install folder only shows the
release link.

## Rejected alternatives
- Checking automatically by default: a request the user did not ask for, against "everything local".
- The GitHub API: 60 requests an hour without a login, shared with everything else on the machine.
- Running `install.sh` from the main branch: trusts a script fetched at update time instead of
  the checksummed release archive.
- Leaving restart to the service manager: `Restart=on-failure` does not restart a clean exit, and
  a manual `quipu ui` has no manager at all.

## Consequences
The only request quipu makes on its own goes to github.com and carries nothing about the user.
A failed install leaves the old quipu in place: it is replaced last.
The release archive layout is now an interface: changing it needs the updater changed too.
