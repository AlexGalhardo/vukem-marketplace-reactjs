# Vukem Marketplace

A mini marketplace inspired by Mercado Livre's layout (not affiliated, no brand assets), built with the [vukem](https://github.com/AlexGalhardo/vukem) Go framework, Inertia v3, React 19, Base UI and Tailwind v4. Live (soon): <https://marketplace.vukem.dev>.

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

## Contas de desenvolvimento

`docker compose up -d --build` runs a one-shot `seed` service after `app` is healthy (`media:bucket` + `db:seed`); on the host run `vukem db:seed` after `vukem migrate`. Seeding is idempotent (re-runs create nothing new) and refuses `APP_ENV=production`, even with `--force`: known-password accounts never exist there.

| Role | Email | Password (default) | Notes |
| --- | --- | --- | --- |
| Buyer | `buyer@example.com` | `marketplace-demo-password` | |
| Seller | `seller@example.com` | `marketplace-demo-password` | owns "Loja Demo Vukem" (12 products) |
| Admin | `admin@example.com` | `marketplace-demo-password` | 2FA (TOTP) is mandatory and already confirmed |

- The password is `SEED_PASSWORD` from `.env` (at least 12 characters) when set.
- Admin 2FA: the seeder logs the TOTP secret and the recovery codes once, when it creates the admin. Read them with `docker compose logs seed | grep totp_secret` (host: the `vukem db:seed` output), add the secret to an authenticator app, then sign in. Lost it? Run `vukem migrate:fresh --seed` (wipes the local database) to get a new one.
- Demo catalog (`vukem demo:catalog`, also run by `db:seed`; deterministic, pt-BR):
  - 100 sellers (`*@vendedores.example.com`) with one shop each across the 27 UFs and 2000 published products (10 to 30 per shop) with shared placeholder images;
  - 300 buyers (`*@compradores.example.com`, verified, one address each); sellers and buyers get random passwords nobody knows and are not meant to log in;
  - about 1500 simulated orders over the last six months in the real order tables (card or PIX, sometimes several items or shops; mostly delivered, some shipped, paid or canceled; fee from `PLATFORM_FEE_BPS`), stock lowered accordingly (restocked first when it would go below zero);
  - about 1200 reviews of paid, shipped or delivered lines (mostly 4 and 5 stars, some seller replies and "helpful" votes; product ratings recalculated) and about 1200 product questions, most answered by the shop.
  - The orders are written directly: no Stripe or AbacatePay call, email, webhook, realtime event, queued job or ledger entry (demo shops show sales but a zero balance, so nothing can be paid out).
  - Re-running adds only what is missing (sellers, buyers, and the activity of buyers that have none) in about a second; a cold run takes 10 to 20 seconds locally.
  - `vukem demo:catalog --fresh` deletes the demo sellers and buyers and everything hanging off them, then seeds again. It never deletes real users' rows: when real data references the demo catalog (a real order, review, question, vote or cart item on a demo product, a ledger entry or payout of a demo shop) it refuses, says what blocks it, and deletes nothing.
- Production: the first admin is an existing account promoted with `users:make-admin <email>` (2FA is then required). The demo catalog is never seeded there unless explicitly requested: `vukem demo:catalog --allow-production` (inside the production container, e.g. a `railway ssh` shell: `/app/app demo:catalog --allow-production`; add `--fresh` to rebuild it). Its sellers and buyers have random passwords.

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
