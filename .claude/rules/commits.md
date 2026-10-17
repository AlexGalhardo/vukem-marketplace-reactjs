# Commit rules

- Conventional Commits 1.0, English: `type(scope): description`.
- Types: feat, fix, perf, refactor, docs, test, build, ci, chore, style, revert.
- Scope = area, lowercase: `auth`, `catalog`, `cart`, `checkout`, `payments`, `orders`, `shops`, `admin`, `api`, `ui`, `infra`, `deps`, `repo`, `docs`.
- Description: imperative, lowercase start, no trailing period, header ≤ 100 chars.
- Breaking change: `!` after type/scope plus a `BREAKING CHANGE:` footer.
- Reference the PLAN.md task in the body: `Refs: F.2.3`.
- One logical change per commit. Validated by the `commit-msg` hook (`bunx commitlint --edit`) and by CI on pull requests.
