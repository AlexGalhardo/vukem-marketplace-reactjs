# Deploy on Railway

Production runs at <https://marketplace.vukem.dev> on Railway from the Docker image built by `release.yml` (GHCR) or from the repository `Dockerfile`.

## Services

| Service | Source | Start command |
| --- | --- | --- |
| `app` | repo root `Dockerfile` | image default (migrates, syncs reference data, serves) |
| `worker` | same image | `/app/app queue:work` |
| `scheduler` | same image | `/app/app schedule:work` |
| `client-logger` | `services/client-logger/Dockerfile` | image default (Bun) |
| Postgres, Redis, MongoDB | Railway plugins | n/a |
| Bucket | Railway bucket | n/a |

Expose port 3000 of `app` on the public domain and 50051 through a TCP proxy for gRPC. Set the healthcheck path to `/ready`.

## Variables

Set on `app`, `worker` and `scheduler` (use Railway references for the plugin URLs):

| Variable | Notes |
| --- | --- |
| `APP_ENV=production`, `APP_URL`, `APP_KEY` | key from `vukem key:generate`; never reuse the local one |
| `DATABASE_URL`, `REDIS_URL`, `MONGO_URL` | from the plugins |
| `S3_*` | from the bucket |
| `STRIPE_*`, `ABACATEPAY_*`, `SOCIAL_GOOGLE_*`, `MAILTRAP_*`/`SMTP_*` | see [PLAN.md section 7](../PLAN.md#7-developer-actions-owner-to-do) |
| `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `TELEGRAM_LOG=true`, `TELEGRAM_MIN_LEVEL=warn` | ops channel, see [telegram-logging.md](telegram-logging.md) |
| `CLIENT_LOGGER_URL`, `CLIENT_LOGGER_KEY` | URL of the `client-logger` private domain (`http://client-logger.railway.internal:3100`) and the shared secret |
| `TRUSTED_PROXIES`, `CLIENT_IP_HEADER`, `TRUST_FORWARDED_PROTO` | keep the edge range only |

`client-logger` needs `CLIENT_LOGGER_KEY` (same value), `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `APP_ENV=production` and `PORT=3100`. Do not give it a public domain: only `app` calls it.

## Steps

1. Create the project, the plugins and the bucket.
2. Create `app`, `worker`, `scheduler` and `client-logger` and set their variables.
3. Deploy; `app` runs the migrations and `reference:sync` on every start.
4. Point the DNS CNAME for the domain at Railway and wait for the certificate.
5. Register the Stripe and AbacatePay webhook endpoints (`/webhooks/stripe`, `/webhooks/abacatepay`).
6. Check `https://marketplace.vukem.dev/ready`.

## Demo catalog in production

Production never creates the demo accounts. The demo catalog (shops, products, reviews) is opt-in:

```sh
railway run /app/app demo:catalog --allow-production
```

Its images are embedded in the binary, so they survive redeploys and need no bucket objects.

## After a deploy

- `/ready` returns ok and a product page shows its image.
- Open the site, run `setTimeout(() => { throw new Error("deploy check") })` in the browser console, and check that the chat receives a `browser` message.
