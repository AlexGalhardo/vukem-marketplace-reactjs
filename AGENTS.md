# AGENTS.md

Vukem Marketplace: a Mercado Livre-inspired marketplace on the vukem Go framework (Inertia v3 + React + Base UI + Tailwind v4). Module `github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs`.

## Start here

1. Read [PLAN.md](PLAN.md): briefing, decisions, architecture; pick the first unchecked task of the current phase.
2. Read the `.claude/rules/*` that match the files you touch.
3. Test first; tick the task only when its `→ verify:` line passes; add a `CHANGELOG.md` line under `[Unreleased]`.

## Commands

- Go: `golangci-lint fmt`, `golangci-lint run ./...`, `go vet ./...`, `go test -race ./...`, `go tool govulncheck ./...`
- Integration: `go test -tags integration ./tests/integration/...` with `TEST_DATABASE_URL`, `TEST_REDIS_URL`, `TEST_MONGO_URL`
- Web: `bun run lint`, `bun run typecheck`, `bun run build`, `bun run test:e2e` (stack on :3200)
- Stack: `docker compose up -d --build`; probes `curl localhost:3200/health` and `/ready`

## Rules

- vukem is pinned by tag, never `replace`. Framework bugs, gaps and docs issues go to `../framework-feedback/marketplace.md`; workarounds are tagged `// workaround(vukem): <title>`. Upgrade flow: [PLAN.md section 9.3](PLAN.md#93-framework-upgrade-flow).
- Money: `int64` cents, server-side pricing, a ledger row per money event, idempotent webhooks (`.claude/rules/money.md`).
- UI strings live in `resources/lang/{en,pt-BR}.json` only; both files keep the same keys.
- JS via Bun only; exact dependency pins; no new dependency without the owner's OK.
- Conventional Commits in English; push to `main`, then `gh run list` and fix CI before new work.
- Never commit `.env` or `.claude/settings.local.json` (owner-local agent settings, currently `{"disableAllHooks": true}`).
- Never create git worktrees without explicit owner confirmation.
- Everything (code, comments, docs, commits) in English.
