# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Foundation: vukem app with Inertia v3, React 19, Base UI and Tailwind v4; home page with dark/light toggle and EN/PT-BR locale switch sharing one set of JSON catalogs with the server.
- Postgres, Redis (sessions, cache, queues) and MongoDB wiring; `/health` (liveness) and `/ready` (dependency checks) probes outside the session stack.
- First migration enabling `citext` and `pg_trgm`.
- Docker multi-stage image and `compose.yaml` (app, postgres, redis, mongo, rustfs, mailpit).
- Tests: unit, HTTP, integration (`-tags integration`) and Playwright smoke e2e.
- Tooling: golangci-lint, Biome, markdownlint, commitlint, lefthook hooks, `.editorconfig` with tabs.
- GitHub Actions: CI (Go, web, docs, commits, integration, e2e), DAST (ZAP and nuclei), release (GHCR image and GitHub release), Dependabot.
- Authentication from `vukem install:auth --2fa --social google`: register, login, logout, password reset, email verification, TOTP 2FA with recovery codes, Google sign-in; Base UI form fields; header auth links.
- `users:make-admin` command, `role` column and an `/admin` area that requires the admin role and confirmed 2FA.
- Mailtrap Email Sandbox test helper and an integration test that follows the real verification email.
- Schema for the catalog, commerce (cart, freight, orders, ledger, payouts, provider events, reviews) and API (keys, outgoing webhooks, contact) tables, with Postgres full-text and trigram search on products.
- PLAN.md with architecture, data model, money flows, OWASP Top 10:2025 checklist and roadmap.

### Changed

- vukem bumped to v0.23.0 and `drivers/postgres` to v0.4.0 (outgoing webhooks, `contrib/imaging`, shared rate limits via `RATE_LIMIT_STORE`, `key:generate`).
- MongoDB database name comes from `MONGO_DATABASE`, else the `MONGO_URL` path, else `marketplace`, so Railway's plugin URL (no path) works unchanged.
- vukem bumped to v0.22.0: read-only visits no longer create sessions, and `vukemtest` sends the asset version from `Inertia("")` (local test helper removed).

### Security

- Login, 2FA and auth-mail rate limits are stored in Redis, so they hold across instances.
- The 2FA QR code is rendered as a `data:` image instead of raw HTML.
- Users get 10 two-factor recovery codes (was 8).
- New accounts always get the `user` role; a `role` sent in the registration body is ignored.

[Unreleased]: https://github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/commits/main
