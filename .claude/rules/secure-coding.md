---
paths:
  - "**/*.go"
  - "PLAN.md"
---

# Security rules

- The OWASP checklist in PLAN.md is a contract: each item names the test that proves it.
- Secure by default; unsafe behaviour is an explicit opt-out with an "Insecure" name, never a missing opt-in.
- Fail closed: if a check cannot run (store down, policy missing, key missing), deny.
- Misconfiguration that weakens security is a boot error, not a warning.
- SQL: bound parameters only; identifiers validated against model metadata; never concatenate input into SQL.
- HTML via `html/template`; never wrap user input in `template.HTML`.
- Randomness from `crypto/rand`; compare secrets with `subtle.ConstantTimeCompare` / `hmac.Equal`.
- Never log secrets, tokens, passwords, `Authorization` or `Cookie` headers; use `config.Secret`.
- Client-facing errors never include stack traces, SQL, paths or config (CWE-209).
- File access through `os.Root`; outbound requests to user-supplied URLs through `httpclient.Safe()`.
- Load the `golang-security` skill for any change in accounts, payments, uploads, webhooks or API keys.
