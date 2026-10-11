# quipu

See what Claude Code knows about each of your projects and what it costs.

- What gets loaded into every session, what on demand, and how many tokens it takes.
- Cache hit rate by project, profile and day, and likely reasons for misses.
- Rules and knowledge repeated across projects and subscriptions, with fixes as diffs.
- Several subscriptions (personal, work) with one shared library of skills and rules.

Everything runs locally; nothing leaves your machine.

> Early development. See [docs/DESIGN.md](docs/DESIGN.md).

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/axvilab/quipu/main/install.sh | sh
quipu install    # start at login, add to the application menu (undo: quipu uninstall)
quipu ui         # open http://quipu.localhost:47800
```

The script puts one binary into `~/.local/bin` and checks its checksum; no root needed.
Linux (glibc, x86_64/arm64) and macOS (Intel/Apple Silicon).

Try it on a fake home first, so nothing of yours is touched:

```sh
quipu demo /tmp/qdemo
HOME=/tmp/qdemo quipu ui
```

CLI: `quipu cache`, `quipu map [path]`, `quipu advise [path]`, `quipu daemon`.


## Development

Go runs only in Docker, so nothing gets installed on your machine:

```sh
make test     # run tests
make build    # produces bin/quipu
make clean    # removes bin/
make clean-cache  # also removes the Docker volume with Go caches
```

Want a shorter command? `alias qp=quipu`

## License

MIT
