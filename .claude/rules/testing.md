---
paths:
  - "**/*_test.go"
  - "tests/**"
  - "e2e/**"
---

# Testing rules

- Test first. A PLAN.md task is done only when its `→ verify:` line passes.
- Go: stdlib + `github.com/google/go-cmp/cmp`, table-driven, `t.Parallel()` when safe, `t.Context()`, no testify.
- Unit tests (`go test ./...`) never touch the network or Docker; `vukemtest.New` gives an in-memory SQLite app for HTTP-level tests that do not need Postgres-only SQL.
- Integration tests carry `//go:build integration` and read `TEST_DATABASE_URL`, `TEST_REDIS_URL`, `TEST_MONGO_URL` (compose or CI service containers). Skip with a clear message when unset.
- Money, auth and webhooks need an abuse test next to the happy path (see PLAN.md, OWASP checklist).
- E2E: Playwright in `e2e/`, against `docker compose` or the CI stack. Email assertions go through the Mailtrap Sandbox API helper; those specs skip when `MAILTRAP_API_TOKEN` is unset and run serially (sandbox rate limit).
- Time-dependent code takes a clock; never `time.Sleep` in tests.
