# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Foundation: vukem v0.21.1 app with Inertia v3, React 19, Base UI and Tailwind v4; home page with dark/light toggle and EN/PT-BR locale switch sharing one set of JSON catalogs with the server.
- Postgres, Redis (sessions, cache, queues) and MongoDB wiring; `/health` (liveness) and `/ready` (dependency checks) probes outside the session stack.
- First migration enabling `citext` and `pg_trgm`.
- Docker multi-stage image and `compose.yaml` (app, postgres, redis, mongo, rustfs, mailpit).
- Tests: unit, HTTP, integration (`-tags integration`) and Playwright smoke e2e.
- Tooling: golangci-lint, Biome, markdownlint, commitlint, lefthook hooks, `.editorconfig` with tabs.
- GitHub Actions: CI (Go, web, docs, commits, integration, e2e), DAST (ZAP and nuclei), release (GHCR image and GitHub release), Dependabot.
- Authentication from `vukem install:auth --2fa --social google`: register, login, logout, password reset, email verification, TOTP 2FA with recovery codes, Google sign-in; Base UI form fields; header auth links.
- PLAN.md with architecture, data model, money flows, OWASP Top 10:2025 checklist and roadmap.

### Security

- Login, 2FA and auth-mail rate limits are stored in Redis, so they hold across instances.
- The 2FA QR code is rendered as a `data:` image instead of raw HTML.

[Unreleased]: https://github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/commits/main
