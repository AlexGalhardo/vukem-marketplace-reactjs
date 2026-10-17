# Vukem Marketplace

A mini marketplace inspired by Mercado Livre's layout (not affiliated, no brand assets), built with the [vukem](https://github.com/AlexGalhardo/vukem) Go framework, Inertia v3, React 19, Base UI and Tailwind v4. Live (soon): <https://marketplace.vukem.com>.

Buyers browse shops, fill a multi-seller cart and pay with card (Stripe Connect Express, test mode) or PIX (AbacatePay sandbox). Sellers list products and ship orders; admins moderate. It is also the flagship battle test for vukem 1.0.

> Status: Foundation done (app skeleton, i18n EN/PT-BR, dark/light theme, Postgres/Redis/MongoDB wiring, probes, CI). Roadmap in [PLAN.md](PLAN.md).

## Stack

| Layer | Choice |
| --- | --- |
| Backend | Go 1.27, vukem v0.22.0 |
| Frontend | Inertia 3.9, React 19.3, TypeScript 7, Base UI 1.9, Tailwind 4.3, Vite 8 |
| Data | PostgreSQL 18 (core), Redis 8 (sessions, cache, queues, rate limits), MongoDB 8 (audit, telemetry, history) |
| Payments | Stripe Connect Express (card, 10% fee), AbacatePay PIX |
| Email | SMTP to the Mailtrap Email Sandbox (Mailpit offline) |
| Tooling | Bun 1.4, Biome, Playwright, golangci-lint, lefthook, commitlint |
| Hosting | Railway (Docker image) |

## Quick start

Requirements: Go 1.27.2, Bun 1.4.3, Docker, and the vukem CLI (`go install github.com/AlexGalhardo/vukem/cmd/vukem@v0.22.0`).

```sh
cp .env.example .env
vukem key:generate              # writes APP_KEY to .env
bun install                     # JS deps + git hooks
docker compose up -d --build    # app on http://localhost:3200 + all services
curl http://localhost:3200/ready
```

Develop on the host with hot reload:

```sh
docker compose up -d postgres redis mongo mailpit rustfs
vukem migrate
vukem serve                     # http://localhost:3000, Vite HMR
```

| Service | URL |
| --- | --- |
| App (compose) | <http://localhost:3200> |
| Mailpit | <http://localhost:8035> |
| RustFS console | <http://localhost:9201> |
| Postgres / Redis / Mongo | `localhost:5442` / `6389` / `27027` |

## Checks

```sh
golangci-lint run ./... && go test -race ./...
bun run lint && bun run typecheck && bun run build
bun run test:e2e                # needs the compose stack and `bunx playwright install chromium`
```

Integration tests and the email-testing setup (Mailtrap) are described in [PLAN.md](PLAN.md#91-commands).

## Documentation

- [PLAN.md](PLAN.md): architecture, data model, money flows, routes, APIs, OWASP checklist, roadmap
- [AGENTS.md](AGENTS.md) and [CLAUDE.md](CLAUDE.md): rules for coding agents
- [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), [CHANGELOG.md](CHANGELOG.md)

## License

[MIT](LICENSE), Alex Galhardo
