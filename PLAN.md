# PLAN — vukem-micro-mercadolivre-reactjs

Main onboarding file for every session and agent. Read this first, then `AGENTS.md`. Pick the first unchecked task of the current phase, do it test-first, run its `→ verify:` line, tick it, commit.

## 1. Briefing

A mini marketplace inspired by Mercado Livre's layout (never its brand), live at <https://marketplace.vukem.com>. Buyers browse shops and products, fill a multi-seller cart, pay with card (Stripe Connect Express) or PIX (AbacatePay), and follow their orders in real time. Sellers open a shop, list products with images, ship orders and get paid. Admins moderate.

It is also the **flagship battle test for vukem 1.0** (<https://github.com/AlexGalhardo/vukem>, docs <https://vukem.vercel.app>): every vukem bug, missing feature, awkward API, security issue or docs problem goes to `../framework-feedback/marketplace.md` (rule: `.claude/rules/framework-feedback.md`), with a local workaround tagged `// workaround(vukem): <entry title>`.

Previous version: the owner's Nuxt 4 marketplace (resell.sh, v1.1.0, read-only at `../_reference-nuxt-marketplace`). This project reaches feature parity with it (section 11 maps every feature) and adds auth hardening, PIX, i18n, three data stores, three API styles and the framework showcase items.

## 2. Decision log

| # | Decision | Why |
| --- | --- | --- |
| D1 | Backend vukem (Go), pinned by tag (started at `v0.21.1`), no `replace` | Battle-test the released framework, like any user |
| D2 | Frontend Inertia **v3** (stable: `@inertiajs/react` 3.9.1 is `latest` on npm, 2026-10-17) + React 19 + TypeScript strict | Server-driven pages, no client router or API layer for pages |
| D3 | Base UI (`@base-ui/react` 1.9.0, headless) styled with pure Tailwind v4; no shadcn | Accessible behaviour, our own look |
| D4 | i18n EN + PT-BR from one set of JSON catalogs (`resources/lang`) read by vukem i18n and imported by React | No drift between server and client strings |
| D5 | Dark/light toggle: `.dark` class on `<html>`, set before paint by a nonce'd inline script | No flash, strict CSP |
| D6 | Postgres = core data; Redis = sessions, cache, queues, rate limits, pub/sub; MongoDB = audit log, telemetry events, product view/search history | Each store does what it is good at; Mongo data is append-only and non-critical |
| D7 | Card: Stripe Connect **Express** sellers, **separate charges and transfers** (one Checkout for a multi-seller cart), platform fee 10% (`PLATFORM_FEE_BPS=1000`) on items, not shipping | Multi-seller carts; same model as the Nuxt reference, which works |
| D8 | PIX: AbacatePay Checkout Transparente at checkout, money lands on the platform; per-seller ledger + **manual payout** by admin | AbacatePay has no marketplace split; ledger keeps it auditable |
| D9 | Currency BRL, money as `int64` cents | PIX is BRL-only; integers avoid rounding bugs |
| D10 | Shipping: ViaCEP address autofill + simulated freight table by weight and destination UF; no Correios contract | Realistic UX without a carrier account |
| D11 | Email: vukem SMTP driver → **Mailtrap Email Sandbox** for dev/staging/e2e; tests read delivered mail through the Mailtrap Sandbox API; Mailpit stays in compose for fully offline work; production transport switchable by env | Resend needs DNS setup; Mailtrap gives an inspectable inbox with an API (owner, 2026-10-17) |
| D12 | Auth: email/password, Google, password reset, email verification, TOTP 2FA with 10 recovery codes (optional for users, **mandatory for admins**) | Owner scope; closes the Nuxt "no MFA" accepted risk |
| D13 | APIs: REST v1 (OpenAPI + Scalar), GraphQL (`contrib/graphql`), gRPC on a separate port behind a Railway TCP proxy; auth by API keys or session | Showcase all three; same service layer underneath |
| D14 | Observability: OpenTelemetry traces + metrics over OTLP to Railway tracing, JSON logs (slog), `/health` + `/ready` | Railway-native |
| D15 | Telegram bot: contact form messages and rate-limited 5xx alerts | Cheap ops channel |
| D16 | Images: server-side upload via vukem upload → S3 (Railway bucket; RustFS locally), type/size/pixel caps, resize to fixed widths | Presigned PUT is not bound to size/type in vukem v0.21.1 (audit S8) |
| D17 | Local object storage is **RustFS** (`rustfs/rustfs:1.0.2`), not MinIO | MinIO stopped publishing images (repo archived) |
| D18 | JS tooling: Bun 1.4.3, Biome 2.5.15, Playwright 1.64.1; Go 1.27.2, golangci-lint 2.15.0 | Latest stable on 2026-10-17, exact pins |
| D19 | Hosting: Railway (app + Postgres + Redis + Mongo + bucket), Docker image from this repo | Owner choice |
| D20 | Framework upgrades: after each vukem release the coordinator sends the tag; bump, drop obsolete workarounds, mark feedback `Status: fixed in vX.Y.Z` (section 9.3) | Owner policy 2026-10-17 |
| D21 | Base UI `CSPProvider` gets the request nonce; Inertia progress bar gets it too | vukem CSP `style-src` is nonce-only (audit S11) |
| D22 | `/health` and `/ready` answered by server-level middleware, not routes | Routes would create a session per probe (feedback entry) |

## 3. Architecture

### 3.1 Versions (verified 2026-10-17, exact pins)

| Layer | Version |
| --- | --- |
| Go toolchain | 1.27.2 |
| vukem | v0.21.1 (+ `drivers/postgres` v0.3.0, `drivers/redis` v0.1.0, `drivers/mongo` v0.1.0; later `drivers/s3`, `drivers/otel`, `contrib/graphql`, `contrib/grpc` v0.1.0) |
| pgx / go-redis / mongo-driver | v5.11.0 / v9.23.0 / v2.9.2 |
| stripe-go | v87.0.0 (`github.com/stripe/stripe-go/v87`, Payments phase) |
| Bun | 1.4.3 |
| React / Inertia / Base UI | 19.3.1 / 3.9.1 (`@inertiajs/react`, `@inertiajs/vite`) / 1.9.0 |
| Vite / Tailwind / TypeScript | 8.3.4 / 4.3.3 / 7.0.2 |
| Biome / Playwright / commitlint / lefthook / markdownlint-cli2 | 2.5.15 / 1.64.1 / 21.2.3 / 2.2.2 / 0.23.3 |
| Images | `postgres:18.6-alpine`, `redis:8.10.2-alpine`, `mongo:8.3.13`, `rustfs/rustfs:1.0.2`, `axllent/mailpit:v1.31.6`, `oven/bun:1.4.3-alpine`, `golang:1.27.2-alpine`, `alpine:3.24.2` |
| Actions | checkout v7.0.1, setup-go v7.0.0, setup-bun v2.2.0, golangci-lint-action v9.3.0, upload-artifact v7.0.2, build-push-action v7.4.0, zaproxy/action-baseline v0.15.0, nuclei-action v3.1.0, action-gh-release v3.0.3 |

### 3.2 Folder layout

```text
cmd/app/main.go             vukem.Main(bootstrap.New): server by default, or a command (migrate, route:list…)
bootstrap/                  app.go (all wiring, no magic), providers.go, commands.go
app/
  platform/                 infra clients (Redis, Mongo) + readiness registry
  controllers/              Inertia page handlers, probes, locale
  accounts/ (Auth)          users, passwords, 2FA, Google, email verification
  catalog/ shops/ cart/ checkout/ orders/ payments/ shipping/ reviews/ admin/ api/   (later phases)
  jobs/ listeners/          queue jobs and event listeners (registries)
  mail/                     mailables + templates
database/migrations/        Go migrations (Postgres-first; SQLite-safe guards for unit tests)
database/seeders/           demo data (vukem db:seed)
routes/                     web.go, api.go, admin.go, schedule.go; registry.go lists them
resources/
  views/app.html            Inertia root view (nonce, theme script, Vite tags)
  lang/en.json, pt-BR.json  catalogs shared by Go and React
  css/app.css               Tailwind v4 + @theme tokens
  js/app.tsx, layouts/, pages/, components/, lib/ (i18n, theme), types.ts
  public/                   static files; build/ is the Vite output (embedded in the binary)
tests/                      HTTP-level tests (vukemtest, SQLite in memory)
tests/integration/          //go:build integration: real Postgres/Redis/Mongo
e2e/                        Playwright specs (smoke, flows, abuse)
.github/workflows/          ci.yml, dast.yml, release.yml
compose.yaml, Dockerfile    local stack and production image
```

### 3.3 Ports

| Port | What | Local (compose host port) | Railway |
| --- | --- | --- | --- |
| 3000 | HTTP: Inertia pages, REST `/api/v1`, GraphQL `/graphql`, webhooks, SSE | 3200 | public domain `marketplace.vukem.com` |
| 50051 | gRPC (`contrib/grpc`) | 50061 | TCP proxy (`*.proxy.rlwy.net:<port>`) |
| 5432 / 6379 / 27017 | Postgres / Redis / Mongo | 5442 / 6389 / 27027 | private network |
| 9000 / 9001 | RustFS S3 / console | 9200 / 9201 | Railway bucket |
| 1025 / 8025 | Mailpit SMTP / UI | 1035 / 8035 | — (Mailtrap) |

### 3.4 Request flows

- **Page**: browser → server middleware (recover, request id, maintenance, JSON log, secure headers + CSP nonce, probes, static) → route middleware (body limit, session in Redis, CSRF via `http.CrossOriginProtection`, i18n locale, Inertia, shared props `appName/locale/locales/auth`) → controller → service → Postgres → `inertia.Render` (HTML on first visit, JSON on Inertia visits).
- **REST/GraphQL**: `Authorization: Bearer mkt_<prefix>_<secret>` (API key, scoped, owner-checked) or the session cookie; rate limit in Redis; `z` schema in and out; problem+json errors.
- **gRPC**: separate listener, API key in metadata, same services.
- **Async**: controller enqueues a job (Redis queue) → worker (`vukem queue:work`, same binary) → mail / webhook delivery / recalcs. Scheduler (`vukem schedule:work`) for PIX expiry, cleanup, rating recalcs.
- **Realtime**: order status changes publish to Redis pub/sub → SSE stream per user (`/events`) → React hook updates the page.
- **Provider webhooks**: `/webhooks/stripe`, `/webhooks/abacatepay` (CSRF bypass) → verify signature → insert event id (unique) → process in a transaction → mark processed.

### 3.5 Data model

Postgres (core, migrations in `database/migrations`). Ids are `bigint` identity unless noted; public ids exposed in URLs are slugs or UUIDv7 (`public_id uuid`) so sequential ids never leak. Money `*_cents bigint`, currency `BRL`. Timestamps `timestamptz`.

| Table | Key columns | Notes |
| --- | --- | --- |
| users | email citext unique, name, password_hash, role (`user`/`admin`), email_verified_at, google_sub unique null, totp_secret (encrypted), totp_enabled_at, password_changed_at | admins only via CLI `users:make-admin`; admin login requires 2FA |
| two_factor_recovery_codes | user_id, code_hash, used_at | 10 per user, hashed, single use |
| password_reset_tokens / email_verification_tokens | user_id, token_hash unique, expires_at, used_at | 32 random bytes, SHA-256 at rest |
| addresses | user_id, recipient, cep, street, number, complement, district, city, uf, phone, is_default | ViaCEP fills street/district/city/uf |
| shops | owner_id unique, slug unique, name, description, logo_key, banner_key, status (`active`/`suspended`), stripe_account_id, stripe_ready, pix_key (encrypted), ship_from_uf | one shop per user |
| categories | slug unique, name_en, name_pt, parent_id | seeded |
| products | shop_id, category_id, slug unique, title, description, price_cents, stock, weight_grams, status (`draft`/`published`/`archived`/`suspended`), rating_avg, rating_count, search tsvector | public only if published and shop active + payable |
| product_images | product_id, key, width, height, position, alt | variants `w=320,640,1280` |
| cart_items | user_id, product_id, quantity | unique (user_id, product_id); always re-priced |
| orders | public_id, buyer_id, status, payment_method (`card`/`pix`), subtotal/shipping/fee/total cents, shipping_address jsonb snapshot, stripe_checkout_session_id, abacatepay_charge_id, pix_expires_at | status: pending → paid → (partially_)refunded / canceled / expired |
| seller_orders | order_id, shop_id, status (`pending`/`paid`/`shipped`/`delivered`/`refunded`/`canceled`), subtotal/shipping/fee/payout cents, stripe_transfer_id, carrier, tracking_code, shipped_at, delivered_at | one per shop in the order |
| order_items | seller_order_id, product_id, title/price snapshot, quantity, weight snapshot | |
| ledger_entries | type, order_id, seller_order_id, shop_id, amount_cents (signed), provider, provider_ref, created_at | append-only; every money event; seller balance = sum |
| payouts | shop_id, amount_cents, status (`requested`/`paid`/`rejected`), pix_end_to_end_id, paid_by, paid_at | manual PIX payouts by admin |
| provider_events | provider, event_id, type, received_at, processed_at, payload_digest | unique (provider, event_id); payload stays in Mongo |
| reviews | product_id, buyer_id, order_item_id unique, rating 1–5, body | only for paid/shipped/delivered items |
| api_keys | user_id, shop_id, name, prefix unique, secret_hash, scopes, last_used_at, expires_at, revoked_at | max 20 active; owner-scoped revoke |
| webhook_endpoints / webhook_deliveries | shop_id, url, secret (encrypted), events / endpoint_id, event, status, attempts, next_attempt_at, response_code | outgoing seller webhooks, SSRF-safe client |
| contact_messages | name, email, subject, body | also forwarded to Telegram |
| sessions | — | in Redis, not Postgres |

Redis: `mkt:session:*` (sessions), `mkt:cache:*` (catalog pages, categories, freight table, 30–60s), `{mkt:queue}:*` (jobs), `mkt:rl:*` (rate limits: login per email+IP, API per key, checkout, contact), pub/sub `orders:<userID>`, `notifications:<userID>`.

MongoDB (append-only, TTL where noted): `audit_log` (actor, action, target, metadata, ip, at; admin + security events), `telemetry_events` (name, props, user, at; TTL 180d), `product_views` (product, user/anon id, at; TTL 90d), `search_history` (q, filters, results, user, at; TTL 90d), `provider_event_payloads` (raw webhook bodies; TTL 90d). Never pass request JSON as a filter (operator injection).

### 3.6 Money flows

Card (Stripe, test mode):

1. Checkout re-prices the cart from Postgres (never trusts client prices), validates stock and shop payability, snapshots the address, computes per-seller `subtotal`, `shipping` (freight table), `fee = round(subtotal × PLATFORM_FEE_BPS / 10000)`, `payout = subtotal + shipping − fee`.
2. One transaction inserts `orders` + `seller_orders` + `order_items` (`pending`) and a `checkout.created` ledger row.
3. Stripe Checkout Session on the platform account: line items, `transfer_group = order public_id`, `metadata.order_id`, idempotency key `checkout-<order_id>`.
4. Webhook `checkout.session.completed` / `async_payment_succeeded` → conditional `pending → paid`, stock decrement (`stock >= qty` guard; oversell → auto-refund that line), cart cleanup, `payment.succeeded` ledger; then one Transfer per seller order to its Express account (`source_transaction`, key `transfer-<seller_order_id>`), ledger `transfer.created`/`transfer.failed` (retry job).
5. Refund (seller-initiated, full per seller order): Stripe refund `subtotal + shipping` (key `refund-<id>`), transfer reversal (key `reversal-<id>`), ledger rows, buyer email.
6. `account.updated` syncs `shops.stripe_ready`. Disputes are logged.

PIX (AbacatePay sandbox):

1. Same steps 1–2 with `payment_method = pix`.
2. `POST /v2/transparents/create` (`method: PIX`, amount in cents, `externalId = order public_id`) → store charge id, `brCode`, QR image, `expires_at`; the page shows QR + copy-paste and listens on SSE.
3. Webhook (HTTPS; `webhookSecret` query param must equal `ABACATEPAY_WEBHOOK_SECRET`, `X-Webhook-Signature` HMAC checked) → idempotent `pending → paid` as in card step 4, ledger `pix.received` (gross) and per-seller `seller.credit` (`payout`).
4. Scheduler job expires unpaid PIX orders (`pending → expired`) and frees nothing (stock is only decremented on payment).
5. Payouts: seller requests, admin pays via PIX outside the app (or AbacatePay withdraw API later), records the end-to-end id → ledger `payout.paid` (negative). Seller balance = ledger sum, never a mutable column.
6. Dev: `POST /v2/transparents/simulate-payment` drives the e2e flow.

### 3.7 Pages and routes (Inertia)

| Route | Page | Auth |
| --- | --- | --- |
| `/` | Home: hero, categories, deals, recently viewed | public |
| `/search?q&category&min&max&uf&sort&page` | Search results with filters in the URL | public |
| `/c/:slug` | Category | public |
| `/p/:slug` | Product: gallery, price, freight by CEP, stock, seller, reviews | public |
| `/s/:slug` | Shop storefront | public |
| `/cart` | Cart (re-priced, availability flags) | user |
| `/checkout` → `/checkout/address`, `/checkout/payment`, `/checkout/pix/:order`, `/checkout/success` | Checkout steps | verified user |
| `/orders`, `/orders/:public_id` | Buyer orders, realtime status, review | user |
| `/account` (+ `/security`, `/addresses`, `/api-keys`, `/notifications`) | Profile, password, 2FA, sessions | user |
| `/sell` | Open a shop (onboarding) | verified user |
| `/seller` (+ `/products`, `/products/new`, `/products/:id/edit`, `/orders`, `/payouts`, `/settings`, `/webhooks`, `/api`) | Seller dashboard | shop owner |
| `/admin` (vukem admin panel) + `/admin/moderation`, `/admin/payouts`, `/admin/ledger` | Admin | admin + 2FA |
| `/login`, `/register`, `/forgot-password`, `/reset-password/:token`, `/verify-email`, `/two-factor-challenge`, `/auth/google`, `/auth/google/callback` | Auth | guest |
| `/contact`, `/terms`, `/privacy`, `/docs/api` (Scalar) | Static / docs | public |
| `/health`, `/ready` | Probes (JSON) | public |
| `/locale` (POST) | Locale cookie | public |

### 3.8 API surface

- **REST v1** `/api/v1` (OpenAPI 3.1 from `z` schemas at `/api/v1/openapi.json`, Scalar at `/docs/api`): `GET products`, `GET products/{slug}`, `GET shops/{slug}`, `GET categories`, `GET freight?cep&product`; `GET|POST cart/items`, `PATCH|DELETE cart/items/{product}`; `GET orders`, `GET orders/{id}`; seller `GET|POST|PATCH shop`, `GET|POST|PATCH|DELETE shop/products[/{id}]`, `POST shop/products/{id}/images`, `GET shop/orders`, `POST shop/orders/{id}/ship|deliver|refund`, `GET shop/balance`, `GET|POST|DELETE shop/webhooks`. Pagination `?page&per_page` → `{data, meta}`; errors RFC 9457 problem+json; idempotency keys on POST (scope = user/API key id, never empty). Hidden or not-owned resources return 404.
- **GraphQL** `/graphql` (`contrib/graphql`, depth/complexity limits): queries `products`, `product`, `shop`, `categories`, `me`, `orders`; mutations `addToCart`, `updateCartItem`, `removeCartItem`. Same services.
- **gRPC** `:50051` (`contrib/grpc`): `catalog.v1.CatalogService` (ListProducts, GetProduct), `orders.v1.OrderService` (ListOrders, GetOrder, WatchOrder stream). API key in `authorization` metadata.
- **Outgoing webhooks** to sellers: `order.paid`, `order.refunded`, `product.suspended`; signed with vukem `webhook.Sign`, delivered by a queue job with exponential retries and a delivery log.
- **Signed URLs** (vukem `signed`): email verification links, order receipt downloads.

## 4. Engineering rules

- Read `AGENTS.md`, `CLAUDE.md` and `.claude/rules/*` (Go, testing, security, money, frontend, commits, changelog, markdown, framework-feedback).
- Test first; a task is done when its `→ verify:` passes. Prefer the narrowest check (one package, typecheck, one spec).
- Conventional Commits in English (scope = area), SemVer tags, Keep a Changelog under `[Unreleased]`.
- `.editorconfig`: tabs width 4 (YAML and Markdown use 2 spaces). Go via `golangci-lint fmt`; TS/JSON via Biome.
- All dependencies latest stable, exact pins (no `^`, no `latest`, no rc/beta). JS via Bun only. No new dependency without the owner's OK.
- Secrets only from env; `config.Secret` in Go; never logged. `.env` and `.claude/settings.local.json` are never committed.
- Server prices everything; client values are hints. Not-owned resources → 404.
- Windows 11 and Linux are both first-class dev machines.

## 5. OWASP Top 10:2025 checklist

Each item names the test that proves it (added in the phase that introduces the surface).

| Id | Risk | Controls | Proof |
| --- | --- | --- | --- |
| A01 | Broken Access Control | policies per resource, owner checks in every query, 404 for not-owned, admin routes need role + 2FA, API key scopes, owner-scoped key revoke | `tests/abuse/idor_test.go` (orders, addresses, products, api keys, webhooks across users) |
| A02 | Security Misconfiguration | vukem secure headers + nonce CSP, `APP_DEBUG` off in prod, HOST/ports explicit, devtools off, CORS deny, ZAP baseline | `e2e/smoke.spec.ts` headers + no CSP console errors; DAST workflow |
| A03 | Software Supply Chain Failures | exact pins, lockfiles, Dependabot, govulncheck, `bun install --frozen-lockfile`, pinned images and actions | CI `go` job (govulncheck), `web` job |
| A04 | Cryptographic Failures | Argon2id/bcrypt via vukem auth, tokens hashed at rest, TOTP secrets and PIX keys encrypted (`orm.Encrypted`), TLS everywhere, `__Host-` cookies in prod | `tests/accounts/*_test.go` (hash at rest), cookie flags test |
| A05 | Injection | bound SQL params only, `z` on every input, Mongo filters from validated fields, React escaping, no `dangerouslySetInnerHTML`, CSV formula neutralisation | `tests/abuse/injection_test.go`, XSS e2e on product title/review |
| A06 | Insecure Design | server-side pricing, conditional state transitions, idempotency keys, stock guard, rate limits on login/checkout/contact | `tests/abuse/price_tampering_test.go`, double-fulfilment test |
| A07 | Authentication Failures | per-email+IP login throttle, generic errors, reset tokens single-use 1h, session rotation on login, invalidate other sessions on password change, TOTP replay protection, recovery codes single use, admin 2FA mandatory | `tests/abuse/auth_bruteforce_test.go`, `tests/abuse/twofactor_bypass_test.go` |
| A08 | Software or Data Integrity Failures | webhook signatures (Stripe, AbacatePay), event-id dedupe, replay window, signed URLs, outgoing webhooks signed | `tests/abuse/webhook_forgery_test.go` (forged, replayed, reordered) |
| A09 | Security Logging and Alerting Failures | JSON logs with request id, security events to Mongo `audit_log`, Telegram 5xx alerts (rate-limited), no secrets in logs | `tests/observability/audit_test.go` |
| A10 | Mishandling of Exceptional Conditions | fail closed when Redis/DB/provider is down, `/ready` 503, panics recovered to 500 without details, timeouts on every outbound call | `tests/abuse/fail_closed_test.go`, `app/platform` readiness tests |

Extra abuse cases: CSRF on every unsafe browser route (cross-site `Sec-Fetch-Site` → 403), SSRF on seller webhook URLs and image URLs (private ranges, redirects, DNS rebinding via `httpclient.Safe()`), upload polyglots (JPEG+HTML, SVG with script, decompression bombs, wrong extension), Mongo operator injection, open redirects on `returnTo`.

## 6. Phases

Ids: `F` foundation, `A` auth, `C` catalog, `K` cart & shipping, `P` payments & checkout, `O` orders, reviews & realtime, `M` admin & moderation, `X` APIs, `B` background & observability, `S` security & QA, `D` deploy & release.

### F — Foundation

- [x] F.1 Repo: `git init -b main`, origin `git@github.com:AlexGalhardo/vukem-micro-mercadolivre-reactjs.git`, MIT LICENSE, README, AGENTS, CLAUDE, CONTRIBUTING, SECURITY, CHANGELOG
  → verify: `git remote -v` shows origin; `bun run lint:md` passes
- [x] F.2 Tooling: `.editorconfig` (tabs 4), `.gitattributes` (LF), `.golangci.yml`, `biome.json`, `.markdownlint-cli2.jsonc`, commitlint, lefthook (pre-commit fmt/lint, commit-msg commitlint, pre-push vet/test/typecheck/govulncheck)
  → verify: `golangci-lint run ./...` = 0 issues; `bun run lint` clean; a bad commit message is rejected
- [x] F.3 Agent setup: `.claude/skills` (Go, security, testing, frontend-design, web-design-guidelines, tdd, karpathy), `.claude/rules` (go, testing, secure-coding, money, frontend, commits, changelog, markdown, framework-feedback); `.claude/settings.local.json` git-ignored
  → verify: `git check-ignore .claude/settings.local.json` prints the path
- [x] F.4 vukem app from `vukem new --inertia react` pinned to v0.21.1, demo posts removed, Postgres driver, translations embedded
  → verify: `go build ./...`
- [x] F.5 Infra wiring: Redis (sessions, cache, queue) before `vukem.New`, Mongo provider, readiness registry; probes `/health` (liveness) and `/ready` (Postgres, Redis, Mongo) outside the session stack
  → verify: `go test ./app/... ./tests/...`; `curl localhost:3200/ready` → `{"checks":{"database":"ok","mongo":"ok","redis":"ok"},"status":"ok"}`
- [x] F.6 Migrations setup: first migration enables `citext` and `pg_trgm` (Postgres-only guard)
  → verify: compose app logs `Migrated: 20261017100000_enable_postgres_extensions`; integration test checks `pg_extension`
- [x] F.7 Frontend: Inertia v3 + React 19 + Base UI + Tailwind v4 tokens, AppLayout (yellow header, search, locale menu, theme toggle), Home page, shared EN/PT-BR catalogs with typed `useT()`, html `lang` sync, `CSPProvider` nonce
  → verify: `bun run typecheck && bun run lint && bun run build`; `go test ./tests/ -run 'Home|Locale|Catalogs'`
- [x] F.8 Docker: multi-stage Dockerfile (Bun assets → Go static binary → alpine non-root, `HOST=0.0.0.0`), `compose.yaml` with app, postgres, redis, mongo, rustfs, mailpit (pinned tags, offset host ports)
  → verify: `docker compose up -d --build --wait app` then `curl localhost:3200/health` = 200
- [x] F.9 Tests: unit (platform readiness), HTTP (home, locale, probes, catalogs parity), integration (`-tags integration`, real services), e2e smoke (probes, headers, no console errors, theme persists, locale switch)
  → verify: `go test ./...`; `TEST_*` env + `go test -tags integration ./tests/integration/`; `bun run test:e2e` = 8 passed
- [x] F.10 CI/CD: `ci.yml` (Go lint/vet/test -race/govulncheck, web Biome/tsc/build, markdownlint, commitlint on PRs, integration with service containers, Playwright e2e on compose), `dast.yml` (ZAP baseline + nuclei, weekly/manual), `release.yml` (tag → GHCR image + GitHub release), Dependabot
  → verify: `gh run list --limit 3` shows CI green on `main`
- [x] F.11 Framework feedback entries for every Foundation finding
  → verify: `grep -c '^## ' ../framework-feedback/marketplace.md` grows; each entry has severity, evidence, workaround

### A — Authentication

- [ ] A.1 Users table + `users:make-admin` command (CLI only), seed demo users
  → verify: `go test ./tests/accounts -run MakeAdmin`
- [ ] A.2 Scaffold with `vukem install:auth --2fa --social google` into a scratch copy, port to Postgres + Inertia React pages with Base UI forms; register/login/logout, session rotation, login throttle (email+IP, Redis)
  → verify: `go test ./tests/accounts -run 'Register|Login|Throttle'`
- [ ] A.3 Email verification (signed link, queued mail through SMTP → Mailtrap), `verified` middleware with redirect for Inertia
  → verify: integration test reads the mail from the Mailtrap Sandbox API and follows the link
- [ ] A.4 Password reset: generic response, async mail, token hashed, 1h, single use; invalidate other sessions on change
  → verify: `go test ./tests/accounts -run Reset`; abuse test for token reuse and user enumeration timing
- [ ] A.5 Google sign-in (vukem social), link by verified email only, PKCE + state, replay-safe in Redis
  → verify: `go test ./tests/accounts -run Google` with fake provider
- [ ] A.6 TOTP 2FA: enable (QR), confirm, 10 hashed recovery codes, challenge after password, replay protection; mandatory for admins (block `/admin` until enabled)
  → verify: `go test ./tests/accounts -run TwoFactor`; abuse: reused code, skipped challenge, recovery code reuse
- [ ] A.7 Mailtrap test helper (`internal/testing/mailtrap`: net/http client, poll by recipient, extract links/codes, clean inbox; skip without `MAILTRAP_API_TOKEN`; serial e2e project)
  → verify: `go test -tags integration ./internal/testing/mailtrap`
- [ ] A.8 Account pages: profile, password, 2FA, active sessions; i18n complete for auth strings
  → verify: e2e `auth.spec.ts` (register → verify mail → login → enable 2FA → logout → challenge)

### C — Catalog

- [ ] C.1 Categories (seeded, EN/PT names), shops (open shop, slug, branding), policies (owner only)
  → verify: `go test ./tests/catalog -run Shop`; IDOR test across two sellers
- [ ] C.2 Products CRUD (draft/publish/archive), visibility rule (published + shop active + payable), slugs
  → verify: `go test ./tests/catalog -run Product`
- [ ] C.3 Image upload: vukem upload sniffing + `image.DecodeConfig` pixel caps + resize 320/640/1280 → S3 (RustFS/Railway bucket), max 8 images, reorder
  → verify: abuse tests (polyglot, SVG, bomb, oversize); `go test -tags integration ./tests/integration -run Upload`
- [ ] C.4 Search: Postgres full text + trigram on title, filters (category, price, UF), sort, pagination, Redis cache, search history to Mongo
  → verify: `go test ./tests/catalog -run Search`
- [ ] C.5 Pages: Home sections, search, category, product (gallery, freight box), shop storefront; product views to Mongo; recently viewed
  → verify: e2e `catalog.spec.ts`

### K — Cart and shipping

- [ ] K.1 Cart (login required), re-pricing on every read, availability flags, own-shop guard, quantity caps
  → verify: `go test ./tests/cart`; price tampering abuse test
- [ ] K.2 ViaCEP client (SSRF-safe fixed host, timeout, cache 24h) + address book
  → verify: `go test ./app/shipping -run ViaCEP` with httptest
- [ ] K.3 Freight table by weight band × destination UF region (seeded), per seller order
  → verify: table-driven `go test ./app/shipping -run Freight`

### P — Payments and checkout

- [ ] P.1 Ledger + provider_events tables, money helpers (cents, fee bps)
  → verify: `go test ./app/payments -run Fee`
- [ ] P.2 Checkout service (re-price, snapshot, one transaction, order public ids)
  → verify: `go test ./tests/checkout`
- [ ] P.3 Stripe Connect Express onboarding (Accounts + Account Links, `account.updated` sync) behind an interface (upstream candidate, audit F5)
  → verify: `go test ./app/payments/stripe` with stripe-mock or recorded fixtures
- [ ] P.4 Card checkout session + webhook fulfilment + transfers + refunds + reversals (idempotency keys)
  → verify: webhook forgery/replay/double-delivery tests; Stripe test card e2e (`4242…`, 3DS, decline)
- [ ] P.5 AbacatePay PIX charge, QR page with SSE, webhook (secret + HMAC), expiry job, simulate-payment e2e
  → verify: `go test ./app/payments/abacatepay`; e2e `pix.spec.ts`
- [ ] P.6 Seller balance from ledger, payout request, admin manual payout record
  → verify: `go test ./tests/payouts`

### O — Orders, reviews, realtime

- [ ] O.1 Buyer orders list/detail, seller orders (ship with carrier + tracking, deliver), emails
  → verify: `go test ./tests/orders`
- [ ] O.2 SSE + Redis pub/sub for order status and notifications; React `useEventStream`
  → verify: e2e status changes without reload
- [ ] O.3 Reviews (eligible items only, one per item, rating recalc job)
  → verify: `go test ./tests/reviews`

### M — Admin and moderation

- [ ] M.1 vukem admin panel over Postgres (users, shops, products, orders) behind admin + 2FA
  → verify: `go test ./tests/admin -run Access`
- [ ] M.2 Moderation actions (suspend/reinstate shop/product with reason), audit log to Mongo, ledger view + CSV export (formula-safe)
  → verify: `go test ./tests/admin`

### X — Public APIs

- [ ] X.1 API keys (`mkt_<prefix>_<secret>`, hashed, scopes, max 20, owner-scoped revoke, per-key rate limit)
  → verify: `go test ./tests/api -run Keys`
- [ ] X.2 REST v1 + OpenAPI + Scalar, idempotency with explicit scope
  → verify: `AssertMatchesOpenAPI` on every endpoint test
- [ ] X.3 GraphQL (`contrib/graphql`) with depth/complexity limits
  → verify: `go test ./tests/graphql`
- [ ] X.4 gRPC (`contrib/grpc`) on `GRPC_PORT`, API key interceptor
  → verify: bufconn tests
- [ ] X.5 Outgoing seller webhooks (signed, queued, retried, delivery log, SSRF-safe)
  → verify: `go test ./tests/webhooks`

### B — Background jobs and observability

- [ ] B.1 Queue worker + scheduler processes (PIX expiry, cleanup of expired tokens/sessions, rating recalcs, transfer retries)
  → verify: `vukem schedule:list` shows tasks; job tests with fake clock
- [ ] B.2 OpenTelemetry (`drivers/otel`) HTTP spans + manual DB/queue/client spans + metrics, OTLP to Railway
  → verify: local OTLP collector receives spans in integration test
- [ ] B.3 Telegram: contact form + rate-limited 5xx alerts
  → verify: `go test ./app/alerts` with httptest Telegram

### S — Security hardening and QA

- [ ] S.1 OWASP abuse suite complete (section 5)
  → verify: `go test ./tests/abuse/...` all green
- [ ] S.2 DAST clean (no medium+ unaccepted findings), exploratory QA checklist in `docs/qa-checklist.md`
  → verify: `gh workflow run dast.yml` report reviewed

### D — Deploy and release

- [ ] D.1 Railway: services (app, worker, scheduler), Postgres, Redis, Mongo, bucket, TCP proxy for gRPC, healthcheck `/ready`, env vars
  → verify: `curl https://marketplace.vukem.com/ready` = ok
- [ ] D.2 DNS `marketplace.vukem.com`, HTTPS, Stripe/AbacatePay webhook endpoints registered
  → verify: provider dashboards show 2xx deliveries
- [ ] D.3 `v1.0.0` release (CHANGELOG, tag, GHCR image)
  → verify: `gh release view v1.0.0`

## 7. Developer actions (owner to-do)

| What | Where it goes | Notes |
| --- | --- | --- |
| Stripe **test** secret + publishable keys; enable Connect (Express); platform account country (BR for BRL card charges) | `STRIPE_SECRET_KEY`, `STRIPE_PUBLISHABLE_KEY` | restricted key preferred; confirm the platform can create Express accounts in test mode |
| Stripe webhook endpoints (platform + Connect) → `https://marketplace.vukem.com/webhooks/stripe` | `STRIPE_WEBHOOK_SECRETS` (comma-separated) | local: `stripe listen --forward-to localhost:3200/webhooks/stripe` |
| AbacatePay sandbox API key (permissions: transparents create/simulate, webhooks) + webhook secret | `ABACATEPAY_API_KEY`, `ABACATEPAY_WEBHOOK_SECRET` | webhook URL must be public HTTPS (staging) |
| Google OAuth client (Web), redirect `https://marketplace.vukem.com/auth/google/callback` and `http://localhost:3200/auth/google/callback` | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | |
| Mailtrap Email Sandbox: API token, account id, sandbox (inbox) id, SMTP credentials | `MAILTRAP_API_TOKEN`, `MAILTRAP_ACCOUNT_ID`, `MAILTRAP_SANDBOX_ID`, `SMTP_USERNAME`, `SMTP_PASSWORD` (host `sandbox.smtp.mailtrap.io:2525`) | also as GitHub Actions secrets for e2e email specs; optional MCP: `claude mcp add mailtrap -e MAILTRAP_API_TOKEN=… -e MAILTRAP_ACCOUNT_ID=… -- npx -y mcp-mailtrap` |
| Production email later: Mailtrap sending domain (CNAME/DKIM/DMARC) or another SMTP | `SMTP_*` | transport is env-only, no code change |
| Telegram bot token (BotFather) + chat id | `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID` | |
| Railway project: app, worker and scheduler services, Postgres, Redis, Mongo, bucket, TCP proxy on 50051, tracing enabled | Railway variables (`DATABASE_URL`, `REDIS_URL`, `MONGO_URL`, `S3_*`, `OTEL_*`, `APP_KEY`, `APP_URL`) | |
| DNS: `marketplace.vukem.com` CNAME to Railway | DNS provider | |
| GitHub: repo secrets for e2e/DAST (`MAILTRAP_*`), GHCR enabled, branch protection on `main` requiring CI | GitHub settings | |

## 8. Status log

- 2026-10-17 — Foundation complete (F.1–F.11). Next: Auth (A.1).

## 9. Operations

### 9.1 Commands

| Task | Command |
| --- | --- |
| Install | `bun install` (also installs git hooks) and `go mod download` |
| Local stack | `docker compose up -d --build` → <http://localhost:3200> |
| Dev server (host) | `docker compose up -d postgres redis mongo mailpit rustfs` then `vukem serve` (Go + Vite HMR on :3000) |
| Migrate | `vukem migrate` (host) — compose app migrates on start |
| Go checks | `golangci-lint run ./...`, `go vet ./...`, `go test -race ./...` |
| Integration | see header of `tests/integration/infra_test.go` for `TEST_*` URLs, then `go test -tags integration ./tests/integration/...` |
| Web checks | `bun run lint`, `bun run typecheck`, `bun run build` |
| E2E | `bunx playwright install chromium` once, stack up, `bun run test:e2e` |
| Vulns | `go tool govulncheck ./...` |

### 9.2 CI

`ci.yml` on push to `main` and PRs; `dast.yml` weekly/manual; `release.yml` on `v*.*.*` tags. After pushing: `gh run list --limit 5`, fix failures before new work.

### 9.3 Framework upgrade flow

When the coordinator announces a vukem release (tag + changelog):

1. `go get github.com/AlexGalhardo/vukem@vX.Y.Z` (and every `drivers/*`, `contrib/*` module in `go.mod` to its new tag), `go mod tidy`.
2. Search `workaround(vukem)` and remove the ones the release fixed.
3. Run `go test ./...`, the integration suite and e2e; push and wait for CI green.
4. Commit `build(deps): bump vukem to vX.Y.Z`; mark the related entries in `../framework-feedback/marketplace.md` as `Status: fixed in vX.Y.Z`.
5. Known heads-up: from the next release `api.Idempotency` refuses keyed requests with an empty `Scope` → always set Scope to the user or API key id.

## 10. Lessons carried from the Nuxt reference

- Use Stripe's async signature verification path in tests; verify against the raw body.
- New Stripe platforms cannot always create v1 Express accounts; readiness is a capability, not `charges_enabled`.
- Last unit oversell: decrement with a `stock >= qty` guard and auto-refund the losing line.
- Two racing webhook deliveries once logged the same transfer twice → unique ledger key per provider ref.
- Dashboard refunds left order state inconsistent → handle `charge.refunded` by reconciling, not only logging.
- Raw webhook payloads became the biggest table → payloads go to Mongo with TTL; Postgres keeps the id + digest.
- Migrations must run in explicit order (vukem registry) — never directory order.
- Paginate every list (orders, reviews, sitemap) from day one.
- Forgot-password leaked existence through timing → send mail off the request path.

## 11. Nuxt feature → vukem/React equivalent

| Nuxt v1.1.0 (resell.sh) | This project |
| --- | --- |
| Nuxt 4 pages + Vue | Inertia v3 pages in React 19 (`resources/js/pages`) |
| Nitro server routes `/api/*` | vukem routes + controllers; REST `/api/v1` with OpenAPI |
| Zod schemas (shared) | vukem `z` on the server; zod in React forms for UX only |
| sealed cookie sessions + passwordVersion | vukem sessions in Redis; session rotation and revoke-others on password change |
| scrypt passwords, reset tokens | vukem auth hashing; hashed single-use tokens; + email verification, Google, TOTP 2FA |
| nuxt-security headers/CSP | vukem `SecureHeaders` + nonce CSP (`CSPProvider`, Inertia progress nonce) |
| rate limits (in-memory/Redis) | vukem ratelimit on Redis (`rc.RateLimiter`) |
| Drizzle + Postgres/SQLite schema | vukem schema migrations + orm on Postgres |
| BullMQ mail queue (optional) | vukem queue on Redis + scheduler |
| Resend email | SMTP → Mailtrap Sandbox (dev/e2e), env-switchable transport |
| Stripe Checkout + Connect (Accounts v2, separate charges & transfers) | same model with stripe-go v87, behind an interface; + AbacatePay PIX + ledger + manual payouts |
| `stripe_events` table | `provider_events` (Postgres) + payloads in Mongo (TTL) |
| `transaction_logs`, `audit_logs` tables | `ledger_entries` (Postgres) + `audit_log` (Mongo) |
| NuxtHub blob images | vukem upload + `drivers/s3` → Railway bucket / RustFS, resized variants |
| digital products + download grants | out of scope v1 (physical goods only); signed URLs used for receipts |
| flat shipping per product | freight table by weight × UF + ViaCEP |
| API tokens `rs_<prefix>_<secret>` | API keys `mkt_<prefix>_<secret>`, REST + GraphQL + gRPC |
| admin pages | vukem admin panel + moderation pages, admin 2FA mandatory |
| Vue Flow system-design page | `/docs/api` (Scalar) + architecture section here |
| lowercase casual EN copy | EN + PT-BR catalogs |
| Vitest + Playwright | Go tests (unit, HTTP, integration, abuse) + Playwright e2e |
| Vercel/NuxtHub deploy | Docker image on Railway |
