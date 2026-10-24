# Architecture and system design

How the marketplace is built, and why. [PLAN.md](../PLAN.md) holds the full decision log, data model and OWASP checklist; this page is the map.

## The big picture

```text
Browser (React 19 + Inertia)
   │  HTML on first visit, JSON on Inertia visits, fetch for search suggestions
   ▼
Go app (Vukem)  ── REST /api/v1 · GraphQL /graphql · gRPC :50051 · webhooks
   │        │            │
   │        │            └── queue jobs ──► worker (same binary: queue:work)
   │        └── scheduler (same binary: schedule:work)
   ▼
PostgreSQL (core data)   Redis (sessions, cache, queues, rate limits, chat fan-out)
MongoDB (audit, history, telemetry)   S3 storage (product and shop images)
```

One Go binary does everything: `serve` (HTTP, gRPC), `queue:work`, `schedule:work` and the one-off commands (`migrate`, `db:seed`, `reference:sync`). Locally and on Railway they run as separate services from the same image.

## Layers

| Layer | Where | Rule |
| --- | --- | --- |
| Routes | `routes/` | Wiring only: URL, name, middleware. |
| Controllers | `app/controllers/` | Parse input with a `z` schema, call a service, render Inertia or JSON. |
| Services | `app/<domain>/` (`catalog`, `accounts`, `payments`, `checkout`, `chat`, ...) | All business rules; money is `int64` cents. |
| Schemas | `app/requests/` | The `z` schemas are the source of truth for validation. |
| Pages | `resources/js/pages/` | One React page per Inertia component; props mirror the controllers. |
| Seeders | `database/seeders/` | Demo data, English only. |

## Key flows

- **Page request**: middleware (recover, request id, secure headers with CSP nonce, session, CSRF via Origin check, locale) → controller → service → Postgres → `inertia.Render`.
- **Search**: `catalog.Service.Search` runs Postgres full text first and falls back to trigram similarity. The navbar box calls `GET /search/suggest?q=` (`catalog.Suggest`): at least 3 characters, the 8 best visible products with image, title, category and price.
- **Images**: uploads go to S3 and a queue job makes the 320/640/1280 variants. Demo products use small WebP files embedded in the binary (`app/catalog/assets/shared`), served by `/media/products/0/...` without touching the disk.
- **Checkout**: server-side pricing, a ledger row per money event, idempotent provider webhooks (Stripe for cards, AbacatePay for PIX).
- **Chat**: Vukem broadcast channels over WebSocket (`/broadcast/ws`) with SSE as fallback, fanned out through Redis.
- **Errors to Telegram**: the Go side uses Vukem's Telegram handler, the browser uses `telegram-logger`; see [telegram-logging.md](telegram-logging.md).

## Validation and limits

Every text input has a minimum and a maximum, checked on both sides. The Go `z` schema is the source of truth; the React form mirrors it for UX (`resources/js/lib/credentials.ts`, `lib/name.ts`).

| Field | Rule |
| --- | --- |
| Name | 4 to 24 characters |
| Email (sign up) | 12 to 48 characters, valid address |
| Password (sign up, reset, change) | 8 to 32 characters with a lowercase letter, an uppercase letter, a number and a special character |
| Other text inputs | `maxLength` 255 by default (`components/Field.tsx`); required means at least 1 character; long texts (questions, reviews, contact) have their own bounds |

Login and "forgot password" accept older, looser values (email up to 255) so existing accounts are never locked out.

## Related docs

- [Local setup with Docker](setup-local-with-docker.md) and [without Docker](setup-local-without-docker.md)
- [Deploy on Railway](setup-deploy-railway.md)
- [Tooling](tooling.md), [Demo data and images](demo-data-and-images.md), [Telegram logging](telegram-logging.md)
