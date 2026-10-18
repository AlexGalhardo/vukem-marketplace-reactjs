# Vukem Marketplace

<p align="center">
  <a href="https://github.com/AlexGalhardo/vukem-marketplace-reactjs/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/AlexGalhardo/vukem-marketplace-reactjs/ci.yml?branch=main&label=tests" alt="Tests"></a>
  <a href="https://github.com/AlexGalhardo/vukem-marketplace-reactjs/actions/workflows/release.yml"><img src="https://img.shields.io/github/actions/workflow/status/AlexGalhardo/vukem-marketplace-reactjs/release.yml?label=ci%2Fcd" alt="CI/CD"></a>
  <a href="https://github.com/AlexGalhardo/vukem-marketplace-reactjs/releases"><img src="https://img.shields.io/github/v/release/AlexGalhardo/vukem-marketplace-reactjs?include_prereleases&label=version" alt="Version"></a>
  <a href="https://github.com/AlexGalhardo/vukem"><img src="https://img.shields.io/badge/built%20with-vukem-00ADD8" alt="Built with Vukem"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/AlexGalhardo/vukem-marketplace-reactjs" alt="License"></a>
</p>

## Introduction

Vukem Marketplace is an open-source multi-seller marketplace built with the [Vukem](https://github.com/AlexGalhardo/vukem) Go framework, Inertia v3, React 19, Base UI and Tailwind CSS v4. Its layout is inspired by the big Latin American e-commerce sites (not affiliated, no brand assets). It runs in production at <https://marketplace.vukem.dev> and is the flagship battle test for Vukem 1.0.

## Summary

| Area | What you get |
| --- | --- |
| **Storefront** | Home sections, Postgres full-text and trigram search with filters and sorting, category, product and shop pages, recently viewed products, EN/PT-BR and dark/light themes. |
| **Sellers** | One shop per seller, products (draft/published/archived) with S3 images and 320/640/1280 thumbnails made by a queue job, orders, balance from a ledger and payout requests. |
| **Cart and checkout** | Server-priced multi-seller cart, address book with ViaCEP autofill, freight table by weight band and region, card through Stripe Connect (Accounts v2, 10% platform fee) and PIX through AbacatePay. |
| **Community** | Reviews of paid order lines with votes, replies and moderation; product questions answered by the shop; live buyer-seller chat over WebSockets with SSE/polling fallback. |
| **Auth** | Email/password, Google sign-in, email verification, password reset, TOTP two-factor with recovery codes, account and security pages; admin area behind the admin role and confirmed 2FA. |
| **Public APIs** | REST `/api/v1` with OpenAPI 3.1 and Scalar docs, GraphQL with depth/complexity limits, gRPC/Connect, scoped API keys and signed, retried, SSRF-safe seller webhooks. |
| **Operations** | Queue worker and scheduler inside `serve`, `/health` and `/ready` probes, OpenTelemetry traces, Telegram alerts for 5xx errors and the contact form, feature flags in `/admin/flags`. |
| **Demo data** | A deterministic pt-BR catalog: 100 shops, 2000 products, 300 buyers, ~1500 orders, ~1200 reviews and ~1200 questions. |

## Why this project

- **A real product, not a demo page.** Money flows, moderation, APIs and operations are complete, so every Vukem feature is tested under real constraints.
- **One domain, three API styles.** REST, GraphQL and gRPC share the same models, scopes and rate limits.
- **Secure by default.** Every item of the [OWASP Top 10:2025](PLAN.md#5-owasp-top-102025-checklist) names the test that proves it.
- **Framework feedback loop.** Every Vukem bug or rough edge found here is reported upstream and fixed in the framework, not hidden in the app.
- **Windows is first-class.** Pure Go, no C compiler; developed on Windows 11, deployed on Linux.

## Stack

| Layer | Choice |
| --- | --- |
| Backend | Go 1.27, Vukem v0.28.0 (`contrib/graphql`, `contrib/grpc`, `contrib/stripe`, `contrib/imaging`, Postgres, Redis, MongoDB, S3, WebSocket and OpenTelemetry drivers) |
| Frontend | Inertia 3.9, React 19.3, TypeScript 7, Base UI 1.9, Tailwind 4.3, Vite 8 |
| Data | PostgreSQL 18 (core), Redis 8 (sessions, cache, queues, rate limits, chat fan-out), MongoDB 8 (audit, history, telemetry), S3-compatible storage (RustFS locally) |
| Payments | Stripe Connect (card, test mode), AbacatePay PIX (sandbox) |
| Email | Mailtrap (Email Sandbox in development and CI, HTTP API in production), Mailpit offline |
| Tooling | Bun 1.4, Biome, Playwright, golangci-lint, lefthook, commitlint |
| Hosting | Railway (Docker image), images on GHCR |

## Quick start

Requirements: Go 1.27.2, Bun 1.4.3, Docker, and the Vukem CLI (`go install github.com/AlexGalhardo/vukem/cmd/vukem@v0.28.0`).

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

## Development accounts

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
- [CHANGELOG.md](CHANGELOG.md): every release, each entry linked to its commit
- [AGENTS.md](AGENTS.md) and [CLAUDE.md](CLAUDE.md): rules for coding agents

## Agentic development

The project is developed with AI coding agents. [AGENTS.md](AGENTS.md), the path-scoped rules in `.claude/rules/` and the skills in `.claude/skills/` describe the conventions, the test-first workflow and the security contract every change follows.

## Contributing

Thank you for considering contributing! Read the [contribution guide](CONTRIBUTING.md) to get started.

## Security vulnerabilities

If you discover a security vulnerability, please follow the [security policy](SECURITY.md) instead of opening a public issue.

## License

Vukem Marketplace is open-source software licensed under the [MIT license](LICENSE).
