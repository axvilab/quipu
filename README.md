# quipu

See what Claude Code knows about each of your projects and what it costs.

- What gets loaded into every session, what on demand, and how many tokens it takes.
- Cache hit rate by project, profile and day, and likely reasons for misses.
- Rules and knowledge repeated across projects and subscriptions, with fixes as diffs.
- Several subscriptions (personal, work) with one shared library of skills and rules.

Everything runs locally; nothing leaves your machine.

> Early development. Nothing works yet. See [docs/DESIGN.md](docs/DESIGN.md).

Linux and macOS.

## Development

Go runs only in Docker, so nothing gets installed on your machine:

```sh
make test     # run tests
make build    # produces bin/quipu
make clean    # removes bin/ and the build cache
```

Want a shorter command? `alias qp=quipu`

## License

MIT
