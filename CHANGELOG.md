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
- Auth mails (verification, password reset) are queued off the request path; `accounts.RequireVerified` middleware redirects unverified users to the notice.
- Account pages: `/account` (name, email change with re-verification) and `/account/security` (password change, 2FA, log out other sessions); EN/PT-BR for every auth page, message and mail.
- Demo users seeder (`db:seed`: buyer, seller, admin with a confirmed TOTP secret logged once; refused in production).
- API keys `mkt_<prefix>_<secret>` (hashed, scoped, max 20 active, expiry, owner-scoped revoke, per-key rate limit) managed at `/account/api-keys`.
- REST `/api/v1` with OpenAPI 3.1 at `/api/v1/openapi.json`, Scalar docs at `/docs/api`, problem+json errors, `{data, meta}` pagination and caller-scoped idempotency keys.
- GraphQL `/graphql` (`contrib/graphql`) with depth, complexity and alias limits.
- gRPC `catalog.v1.CatalogService` and `orders.v1.OrderService` (`contrib/grpc`) on `GRPC_PORT`.
- Catalog: categories, one shop per seller, products (draft/published/archived) with seller dashboard pages, storefront pages (home sections, search, category, product, shop) and recently viewed products.
- Product and shop images uploaded to S3 (RustFS locally) with type sniffing and size/pixel caps; 320/640/1280 variants made by a queue job with `contrib/imaging`.
- Search with Postgres full text and trigram fallback, filters, sorting, a 45 s anonymous page cache and search/view history in MongoDB (90-day TTL).
- Server-priced cart with availability flags and own-shop guard, address book with ViaCEP autofill, freight table by weight band and destination region; add-to-cart and freight quote on the product page.
- Checkout split per seller (UUIDv7 order ids); Stripe Connect Express (onboarding, Checkout, transfers, refunds, reversals); AbacatePay PIX with QR page and expiry job; ledger-based seller balance and manual PIX payouts.
- `media:bucket` command creating the local S3 bucket (refused in production).
- `reference:sync` command (categories, freight table); the image runs `migrate` and `reference:sync` on every start.
- Queue worker and scheduler in the web process by default (`QUEUE_WORKER_IN_PROCESS`, `SCHEDULER_IN_PROCESS`), plus `worker` and `scheduler` compose services; scheduled pruning of expired tokens and old provider events and webhook deliveries.
- OpenTelemetry HTTP, SQL and queue spans over OTLP (off unless `OTEL_EXPORTER_OTLP_*` is set).
- `/contact` form forwarded to Telegram, and rate-limited Telegram alerts for 5xx responses.
- Outgoing seller webhooks at `/seller/webhooks` (vukem `webhook.Sender`, Standard Webhooks signatures, retries, delivery log).
- `/events` server-sent events over Redis pub/sub and a `useEventStream` React hook.
- Accessibility e2e suite (`e2e/a11y.spec.ts`, `@axe-core/playwright` 4.14.0): every page and UI state in light and dark, EN and PT-BR, desktop and mobile, logged out and as buyer, seller and admin; serious and critical WCAG 2.x A/AA violations fail CI.
- OpenTelemetry metrics (HTTP, outbound HTTP, DB, queue) over OTLP, configured by `OTEL_*` env.
- Demo catalog seeder: 100 sellers with one shop each across the 27 UFs and 2000 published pt-BR products (10–30 per shop) with shared placeholder images; deterministic and idempotent; `demo:catalog [--fresh] [--allow-production]` (refuses production without the flag).
- Compose one-shot `seed` service (bucket + `db:seed`) and a README "Contas de desenvolvimento" table for the fixed admin, buyer and seller accounts.
- Admin feature flags at `/admin/flags` (typed, cached 30 s in Redis, defaults on any error, audit log of who/when/old→new): email verification, user 2FA, admin 2FA (locked on), Google login, card checkout, PIX checkout, GraphQL API.
- Continue with Google on login and register (new Google identities create the account).
- Storefront redesign in the big LatAm marketplace style with our own brand: header with deliver-to CEP, banner carousel, shortcut panels, offer shelves, category grid, cards with installments and stars, search filter column with list/grid view, product buy box with buy now, reviews summary and Q&A placeholder, cart summary, footer with a non-affiliation note; light and dark tokens with AA contrast; skip link.
- Product cards carry `rating_avg_x100` and `rating_count`.
- Project-scoped Impeccable design skill (cli-v4.6.0) and zod 4.6.7 for client-side form validation.
- Product questions and answers: buyers ask on product pages (10–500 characters, no contact data, 5 per 10 minutes), sellers answer at `/seller/questions` with a realtime and email notice, admins hide/unhide at `/admin/questions` (audited); `product_questions` flag.
- Product reviews: buyers of paid/shipped/delivered order lines review once (1–5 stars, title, text, recommend); the product page shows the average, count, 5→1 distribution, newest/most-helpful sort, verified-purchase badge, the seller's reply and helpful votes; admins hide/unhide at `/admin/reviews` (audited); `reviews` flag; `reviews:recalculate` command and a daily recalculation task.
- `app/moderation.ContainsContact` shared by questions, answers, reviews and replies (links, emails, handles, phone numbers; CEPs allowed).
- `demo:catalog` simulates activity: 300 demo buyers, about 1,500 orders over six months (card/PIX, multi-item and multi-shop, delivered/shipped/paid/canceled), about 1,200 reviews (votes, seller replies, ratings recalculated) and about 1,200 questions (mostly answered); no provider calls, mails, jobs or ledger rows.
- PLAN.md with architecture, data model, money flows, OWASP Top 10:2025 checklist and roadmap.

### Changed

- vukem bumped to v0.27.0 (`contrib/grpc` v0.4.0, `drivers/otel` v0.3.0): feature flags use vukem `flags` (values migrated from the old `feature_flags` table, admin 2FA locked, audit log kept); `GRPC_INSECURE_CLEARTEXT` replaces `GRPC_INSECURE_PLAINTEXT` (still read); API routes use `auth.ProblemJSON`; Inertia error pages come from vukem's hook (full page loads also render the Error page); Redis stores are built lazily after the config loads (`vukem.ErrUseDefault` without `REDIS_URL` outside production); OTel metrics follow the generic endpoint, so production sets `OTEL_METRICS_EXPORTER=none` (Railway's receiver is traces-only).
- `demo:catalog --fresh` deletes only demo-owned rows and refuses (nothing deleted) when real users' orders, reviews, questions, votes or cart items reference the demo catalog.
- Email verification is off by default (`email_verification` flag); sign-up lands on the dashboard.
- Names must be 4–24 characters with whitespace collapsed and the first letter capitalized (server `z` rule mirrored by zod in the forms).
- vukem bumped to v0.26.0 (`contrib/graphql` v0.3.0, `contrib/grpc` v0.3.0, `contrib/stripe` v0.3.0, `drivers/redis` v0.4.0): Stripe Accounts v2 calls use contrib/stripe's typed helpers (our JSON client removed), SQL built from pieces goes through `database.RebindChecked`, and production mail is documented as `MAIL_MAILER=mailtrap` (Railway blocks SMTP; Mailpit stays local).
- vukem bumped to v0.25.0 (`drivers/otel` v0.2.0, `drivers/redis` v0.3.0, `drivers/s3` v0.3.0, `contrib/graphql` v0.2.0, `contrib/grpc` v0.2.0, `contrib/stripe` v0.2.0). Adopted: the queue worker in serve (`QUEUE_IN_SERVE`, `app/workers` removed), `database.Rebind`, `uuid.NewV7`, app seeders (`WithAppSeeders`), lazy Redis/MongoDB clients (`OpenLazy`) so the app boots while they are down, `/ready` from `app.ReadyHandler` (the database check is now `db`), catalog routes after boot (`WithRoutesAfterBoot`), keyed vukem errors (no English message matching), session versions (`users.session_version`) instead of the password stamp, Stripe v1 calls through `contrib/stripe`, GraphQL Go-name folding, the gRPC server on its own router and port, `ServeOpenAPIWith`, nonce-only CSP on `/docs/api`, `Disk.CreateBucket`, Telegram `SendSync` with `TELEGRAM_LOG=false` (replaces `TELEGRAM_MIN_LEVEL`, `TELEGRAM_API_URL` removed), and DB/queue/mail spans from `drivers/otel` (SQL text is no longer exported).
- Accessibility fixes from the axe scans: text on blue, green and red backgrounds uses theme tokens with AA contrast in both themes (`on-action`, `on-status`, darker success/danger), text links use `action-600`, and the language menu's accessible name keeps its visible code.
- vukem bumped to v0.24.0 (`drivers/s3` v0.2.0, `drivers/mongo` v0.2.0, `drivers/redis` v0.2.0): the scheduler runs inside serve through vukem (`SCHEDULE_IN_SERVE`, replaces `SCHEDULER_IN_PROCESS`), queued mail payloads are sealed with `APP_KEY`, Google sign-in states use vukem's shared `Social.States` store (Redis), and `users:make-admin` prints to stdout.
- Stripe connected accounts use Accounts v2 (Express dashboard, recipient configuration with `stripe_transfers`); new Connect platforms cannot create v1 accounts.
- The locale menu closes after a language is picked.
- vukem bumped to v0.23.0 and `drivers/postgres` to v0.4.0 (outgoing webhooks, `contrib/imaging`, shared rate limits via `RATE_LIMIT_STORE`, `key:generate`).
- MongoDB database name comes from `MONGO_DATABASE`, else the `MONGO_URL` path, else `marketplace`, so Railway's plugin URL (no path) works unchanged.
- vukem bumped to v0.22.0: read-only visits no longer create sessions, and `vukemtest` sends the asset version from `Inertia("")` (local test helper removed).

### Fixed

- Seller-only routes (`/seller/*`, payouts, refunds, webhooks) send users without a shop to `/sell` with a message instead of a raw "Record not found" 404; footer and header show seller links only to shop owners; Inertia visits that fail render the app's Error page (status, safe message) instead of vukem's raw HTML page.

### Security

- Linking Google to an unverified local account also clears its 2FA, API keys, pending reset links, the shop's payout destinations and webhooks; sign-up is rate limited per IP.
- The demo shop seeder refuses `APP_ENV=production` and uses `SEED_PASSWORD` (it could create `seller@example.com` with a fixed password).
- API authentication failures (401/403) are always problem+json, even for `Accept: text/html`; `HTTP_H2C` alone never opens the gRPC port in production.
- Login, 2FA and auth-mail rate limits are stored in Redis, so they hold across instances.
- The 2FA QR code is rendered as a `data:` image instead of raw HTML.
- Users get 10 two-factor recovery codes (was 8).
- Password reset requests reveal nothing about accounts (same response and timing); a password change or reset ends every other session (session stamp) and deletes remember-me and API tokens.
- Google sign-in states are single use across instances (Redis); linking a verified Google email to a never-verified account rotates its password (pre-registration takeover).
- 2FA bypass abuse tests: half-finished login, expired challenge, throttle across restarts, recovery codes bound to their account.
- API keys are compared in constant time with identical failures; GraphQL introspection is off in production; production gRPC is not served without TLS (or an explicit plaintext opt-in behind a TLS proxy).
- Seller routes are owner-scoped (404 for not-owned); uploads are sniffed, capped and re-encoded (drops polyglot payloads and EXIF); media keys are allow-listed and served with `nosniff` and a sandbox CSP.
- Payment webhooks are signature-checked (several Stripe secrets; AbacatePay secret + HMAC) and deduplicated; order transitions are conditional and provider calls carry deterministic idempotency keys; production refuses half-configured payment providers.
- Seller webhook URLs are checked against internal networks at creation and at send time (`httpclient.Safe`); their secrets are encrypted at rest; alerts and contact jobs carry no request data.
- New accounts always get the `user` role; a `role` sent in the registration body is ignored.

[Unreleased]: https://github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/commits/main
