# quipu

See what Claude Code knows about each of your projects and what it costs.

- What gets loaded into every session, what on demand, and how many tokens it takes.
- Cache hit rate by project, profile and day, and likely reasons for misses.
- Rules and knowledge repeated across projects and subscriptions, with fixes as diffs.
- Several subscriptions (personal, work) with one shared library of skills and rules.

Everything runs locally; nothing leaves your machine.

> Early development. See [docs/DESIGN.md](docs/DESIGN.md).

## Try it

```sh
quipu ui                 # web interface for your own Claude Code data
quipu demo /tmp/qdemo    # or a fake home, so nothing of yours is touched:
HOME=/tmp/qdemo quipu ui
```

CLI: `quipu cache`, `quipu map [path]`, `quipu advise [path]`, `quipu daemon`.

Linux and macOS.

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
