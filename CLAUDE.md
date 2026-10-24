# CLAUDE.md

Follow [AGENTS.md](AGENTS.md) and [PLAN.md](PLAN.md). Notes specific to Claude Code:

- Session start: load `karpathy-guidelines`, caveman mode and `graphify` before any task.
- Docs index: see the Docs section of [AGENTS.md](AGENTS.md).
- Skills in `.claude/skills/`: load `golang-*` skills for Go work (`golang-security` for accounts, payments, uploads, webhooks, API keys), `frontend-design` and `impeccable` before UI work (Impeccable is pinned, see `.claude/skills/impeccable/SOURCE.md`; skip its `scripts/impeccable` launcher, which downloads a binary into `~/.impeccable`) and `web-design-guidelines` to audit it, `tdd` for new features, `karpathy-guidelines` always.
- Path-scoped rules in `.claude/rules/` load automatically; `framework-feedback.md` applies everywhere.
- `.claude/settings.local.json` is owner-local (it disables Claude Code hooks) and git-ignored; do not edit or commit it.
- Email in dev and e2e goes to the Mailtrap Email Sandbox. Optional MCP to inspect it: `claude mcp add mailtrap -e MAILTRAP_API_TOKEN=... -e MAILTRAP_ACCOUNT_ID=... -- npx -y mcp-mailtrap` (tests do not need it; they call the Sandbox API).
- Railway MCP/CLI is available for deploy tasks (phase D); confirm destructive actions with the owner.
- Validate with the narrowest command (one package, typecheck, one Playwright spec) before claiming done, and show the command and its result.
