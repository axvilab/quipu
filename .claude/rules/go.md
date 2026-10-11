---
paths:
  - "**/*.go"
---

# Go style

Follow Effective Go, Go Code Review Comments and the Google Go Style Guide. Prefer the obvious,
boring solution a reader understands in one pass.

- Write the concrete code first. Add an interface only when there are two implementations or a
  test needs a fake; define it in the package that uses it, keep it small.
- Accept interfaces, return concrete types. No getters/setters, no constructors that only copy
  fields.
- No frameworks, DI containers, generics or reflection unless the plain version is clearly worse.
  Standard library first; a new dependency needs a reason in the commit message.
- Packages by responsibility (`collect`, `store`, `analytics`), not by layer (`models`,
  `services`, `utils`). No package named `util`, `common`, `helpers`.
- Functions do one thing and fit on a screen. Early returns over nested `if`/`else`.
- Errors: return them, wrap with context `fmt.Errorf("read %s: %w", path, err)`; no `panic` outside
  `main` and impossible states. Don't log and return the same error.
- Names are short and plain in small scopes (`f`, `r`, `i`), descriptive at package level.
  No stutter: `store.Open`, not `store.OpenStore`.
- Comments explain why, not what. Every exported identifier has a doc comment starting with its
  name.
- `context.Context` is the first parameter of anything that does I/O or may block.
- Tests: table-driven where there are several cases, `t.TempDir()` for files, synthetic data in
  `testdata/`. Test behavior through the public API of the package.
