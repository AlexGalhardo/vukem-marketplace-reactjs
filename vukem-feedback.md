# Vukem feedback

Findings from building the marketplace on Vukem (checked against v0.30.0). Each entry has a kind, what happened, the workaround used here and a suggestion. Items marked *idea* were not verified against the framework source.

## Entry format

```text
### <title>
Kind: bug | gap | api | docs | idea
Where: package or docs page
What happened / why it matters
Workaround here: ...
Suggestion: ...
```

## Gaps

### z has no character-class or password-strength rules

Kind: gap · Where: `z.StringSchema`

A sign-up password needs "at least one lowercase, uppercase, digit and special character". `z` only offers `Regex` and `Refine`, and Go's RE2 has no lookahead, so one regex cannot express it; the rule needs a hand-written `Refine` that returns an i18n key.

Workaround here: `requests.PasswordSchema` (`app/requests/password.go`) loops over the runes in a `Refine`.

Suggestion: add `Password(opts)` or rules such as `HasLower()`, `HasUpper()`, `HasDigit()`, `HasSymbol()` that produce the standard `too_small`-style issue codes, so apps can localize them like the built-in rules.

### factory.Faker has no locale, product or company data

Kind: gap · Where: `factory`

Only lorem-ipsum text exists, so the demo catalog (brands, product kinds, review texts, buyer names) lives in hand-written vocabulary files (`database/seeders/demo_catalog_data.go`, `demo_activity_data.go`).

Workaround here: `// workaround(vukem): factory.Faker has no locale or product/company/text data`.

Suggestion: ship small locale packs (names, streets, company and product nouns) for `en` and `pt-BR`, and `factory.Pick` helpers that take a weighted list.

### Seeders receive only `*database.DB`

Kind: gap · Where: `Seeder` interface

A seeder that must write images needs the app config and the media disk. `AppSeeder` solves it, but the two interfaces and the `Name()` hook make it easy to pick the wrong one, and tests call `Run` helpers with hand-built dependencies.

Workaround here: `DemoShop.SeedApp` opens the disk from `app.Source`; `DemoCatalog.Run` no longer needs a disk because its images are embedded.

Suggestion: document `AppSeeder` next to `Seeder` as the default for anything that touches services.

## API ergonomics

### `c.NoContent()` takes no status

Kind: api · Where: `web.Ctx`

`c.JSON(status, v)` and `c.Redirect(...)` take explicit statuses but `c.NoContent()` silently means 204. Writing `c.NoContent(http.StatusNoContent)` (the natural guess) is a compile error.

Suggestion: keep it, but say "204" in the doc comment, or accept an optional status.

### Validation messages mix built-in text and i18n keys

Kind: docs · Where: `z` and `i18n`

`Message("too_small", "account.errors.password_length")` works and the key is localized, while a `Refine` that returns `errors.New(key)` also works, but the docs do not say that refine errors are treated as i18n keys. Discovering it required reading the existing `MsgNameLength` usage.

Suggestion: add a "custom messages and localization" section to the `z` page with both forms.

## Features

### A first-class `critical` / `panic` level for the Telegram handler (*idea*)

Kind: idea · Where: `logging/telegram`

`LevelFatal` exists, but there is no documented way to route recovered panics to a different chat or mark them distinctly from 5xx alerts. Apps currently rely on `Recover` rendering a 500 and an alert middleware wrapped around it.

Suggestion: make `Recover` log at `LevelFatal` with the stack, and document `ChatIDs` routing for it.

### A WebP encoder in `contrib/imaging` (*idea*)

Kind: idea · Where: `contrib/imaging`

The Go standard library cannot encode WebP, so the demo images are drawn in Go and encoded offline with ffmpeg (`internal/tools/genplaceholders`). A pure-Go WebP encode path (even lossless) would let apps and the thumbnail job output WebP without cgo or external tools.

### JSON helpers in `vukemtest` (*idea*)

Kind: idea · Where: `vukemtest`

Testing a JSON endpoint that returns an array needed `Body()` plus manual `json.Unmarshal`. Helpers such as `AssertJSON(cmp)` or `JSONInto(&v)` would shorten these tests.

## Docs

### Local feedback path in the project rule does not exist

Kind: docs · Where: `.claude/rules/framework-feedback.md`

The rule points to `../framework-feedback/marketplace.md`, which is outside the repository and missing on a fresh checkout. This file replaces it.

Suggestion: keep one `vukem-feedback.md` in the repo and have the upstream maintainers read it from here.
