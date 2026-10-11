# The claude shim and quipu doctor

Status: done · 2026-10-12 · finishes module 4 (DESIGN §4.5)

## Shim

So that the plain `claude` command picks the profile by folder, without an alias.

- quipu run as `claude` (argv[0]) behaves like `quipu run`: folder rule → ask → default, then runs
  the real `claude` found on PATH after skipping itself.
- The real `claude` of the native installer is `~/.local/bin/claude`, a link the installer
  rewrites on update, so quipu never touches it. The shim is a symlink `claude` → quipu in quipu's
  own folder (`<data dir>/shim`), put before the rest of PATH by one line in the shell's rc file.
- `quipu profile shim on|off` makes or removes the link; `--rc` also adds or removes the PATH line
  in `~/.bashrc`, `~/.zshrc` or fish's `config.fish` (by `$SHELL`) through the journal. Without
  `--rc` it prints the line. The UI offers the same with a button.

## Doctor

`quipu doctor` and a Check section on the Profiles page list what is wrong, each with how to fix:

- Claude Code found on PATH, its version; several different installs on PATH;
- each profile signed in; symlinks of shared items that point nowhere;
- folder rules for folders that are gone or profiles that do not exist;
- the shim: off, on but not first on PATH, or working;
- the daemon running; the quipu window available (Linux: WebKitGTK);
- `remoteControlAtStartup` on.

## Steps

1. [x] Shim mode in main (claude's arguments passed as they are); `internal/profile` shim helpers
   (link, PATH check, rc block per shell, login shell from /etc/passwd for services);
   `quipu profile shim on|off|status [--rc]`; tests.
2. [x] Doctor checks (Service, `quipu doctor`); API test with shim on and off.
3. [x] UI: shim state and button, Check section on the Settings page; uk and en.
4. [x] Tried on a fake home with a fake claude (work folder → work profile, other folders → default,
   .bashrc line added and removed) and on the demo home in the UI; docs.

The shim's state is judged by the link and the rc line, not by PATH: the quipu service does not
see the shell's PATH.
