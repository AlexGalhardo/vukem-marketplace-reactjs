# AGENTS.md

Vukem Marketplace: a Mercado Livre-inspired marketplace on the vukem Go framework (Inertia v3 + React + Base UI + Tailwind v4). Module `github.com/AlexGalhardo/vukem-marketplace-reactjs`.

## Start here

0. At the start of every session load the `karpathy-guidelines` skill, caveman mode and `graphify` (`graphify query "<question>"` before reading many files; `graphify update .` when the graph is stale).
1. Read [PLAN.md](PLAN.md): briefing, decisions, architecture; pick the first unchecked task of the current phase.
2. Read the `.claude/rules/*` that match the files you touch.
3. Test first; tick the task only when its `→ verify:` line passes; the commit message feeds `CHANGELOG.md` (git-cliff, never edited by hand).

## Commands

- Go: `golangci-lint fmt`, `golangci-lint run ./...`, `go vet ./...`, `go test -race ./...`, `go tool govulncheck ./...`
- Integration: `go test -tags integration ./tests/integration/...` with `TEST_DATABASE_URL`, `TEST_REDIS_URL`, `TEST_MONGO_URL`
- Web: `bun run lint`, `bun run typecheck`, `bun run build`, `bun run test:e2e` (stack on :3200 with `app` and `worker` up and `db:seed` run; includes the axe accessibility scans in `e2e/a11y.spec.ts`, which fail on serious/critical violations; new pages must be added there)
- Stack: `docker compose up -d --build`; probes `curl localhost:3200/health` and `/ready`; demo data `docker compose exec -T app /app/app db:seed` (Git Bash: prefix `MSYS_NO_PATHCONV=1`); reference data only: `reference:sync`
- Tests that need a logged-in user: `internal/testing/authtest.ActingAs` / `SessionCookies` (sessions carry a password stamp; `vukemtest` `ActingAs` alone is logged out)
- Feature flags (`app/flags`, admin page `/admin/flags`): typed keys with defaults, cached 30 s, defaults on any read error. `email_verification` is off by default (sign-up lands on the dashboard); tests that need it call `internal/testing/flagtest.RequireEmailVerification` (or `flagtest.Set`). New on/off features get a flag there, gated server-side and covered by a test
- Reviews: `reviews:recalculate [product ids]` rebuilds `products.rating_avg_x100`/`rating_count` from visible reviews (also a daily scheduled task); `dev:paid-order <buyer-email> <product-slug>` records a paid order without a provider so a local or e2e buyer can review (refuses production)
- Chat (`app/chat`, flag `chat`): vukem broadcast channels `private-chat.{id}`, `presence-chat.{id}`, `private-inbox.{userId}` served on `/broadcast/ws` (drivers/websocket, same-origin only) and `/broadcast/sse` (fallback), fanned out over Redis with the `REDIS_PREFIX` + `chat:` channel prefix; `CHAT_NOTIFY_AFTER` (default `10m`) is the away time before an unread message is emailed; demo threads come from the `DemoChat` seeder (demo accounts only)
- E2E against a local binary: `E2E_DOCKER=0 E2E_APP_BIN=<path to the app binary>` (run Playwright with the app's env so `users:make-admin` reaches the same database); `E2E_GOOGLE=1` when the stack has `SOCIAL_GOOGLE_CLIENT_ID` set (any value)

## Docs

- [docs/architecture-system-design.md](docs/architecture-system-design.md), [docs/tooling.md](docs/tooling.md)
- Setup: [with Docker](docs/setup-local-with-docker.md), [without Docker](docs/setup-local-without-docker.md), [Railway](docs/setup-deploy-railway.md)
- [docs/demo-data-and-images.md](docs/demo-data-and-images.md): English seeds, embedded WebP images (`go run internal/tools/genplaceholders/main.go`)
- [docs/telegram-logging.md](docs/telegram-logging.md): backend alerts (Vukem Telegram) and browser errors (`services/client-logger`, telegram-logger)
- [vukem-feedback.md](vukem-feedback.md): framework findings; append there, tag workarounds `// workaround(vukem): <title>`

## Rules

- vukem is pinned by tag, never `replace`. Framework bugs, gaps and docs issues go to `vukem-feedback.md`; workarounds are tagged `// workaround(vukem): <title>`. Upgrade flow: [PLAN.md section 9.3](PLAN.md#93-framework-upgrade-flow).
- Money: `int64` cents, server-side pricing, a ledger row per money event, idempotent webhooks (`.claude/rules/money.md`).
- UI strings live in `resources/lang/{en,pt-BR}.json` only; both files keep the same keys.
- JS via Bun only; exact dependency pins; no new dependency without the owner's OK.
- Conventional Commits in English; push to `main`, then `gh run list` and fix CI before new work.
- Never commit `.env` or `.claude/settings.local.json` (owner-local agent settings, currently `{"disableAllHooks": true}`).
- Every text input has a minimum and a maximum (Go `z` schema as the source of truth, React form mirrors it): name 4-24, signup email 12-48, password 8-32 with lower, upper, digit and special; `Field` caps other inputs at 255.
- Seeds are English only; demo images are embedded in the binary (`app/catalog/assets/shared`), never remote URLs.
- Never create git worktrees without explicit owner confirmation.
- Everything (code, comments, docs, commits) in English.
