# Telegram logging

Backend and browser problems arrive in the same Telegram chat, with the same bot.

| Side | Library | What is sent |
| --- | --- | --- |
| Backend (Go) | Vukem `logging/telegram`, a Go port of telegram-logger | 5xx alerts (panics are recovered as 500), contact form messages, and logs at or above `TELEGRAM_MIN_LEVEL` when `TELEGRAM_LOG=true` (warning, error, fatal) |
| Browser (React) | [`@galhardoalex/telegram-logger`](https://github.com/AlexGalhardo/telegram-logger) in `services/client-logger` | Uncaught errors and unhandled promise rejections |

The telegram-logger README says to keep the token on the server. So the browser never talks to Telegram: it posts a small JSON report to the app, the app validates and forwards it to the `client-logger` service, and that service uses telegram-logger.

```text
browser ──POST /client-log──► Go app ──POST /log + key──► client-logger (Bun) ──► Telegram
                              (z schema, 10/min per IP)    (telegram-logger, dedupe 10 min)
```

## 1. Create the bot

1. Talk to [@BotFather](https://t.me/BotFather), send `/newbot` and copy the token.
2. Add the bot to your group or channel (admin in a channel).
3. Send a message there and open `https://api.telegram.org/bot<TOKEN>/getUpdates` to read `chat.id`.

## 2. Configure

```sh
TELEGRAM_BOT_TOKEN=123456789:AAH...
TELEGRAM_CHAT_ID=-1001234567890
TELEGRAM_LOG=true            # forward backend logs too
TELEGRAM_MIN_LEVEL=warn      # debug | info | warn | error
CLIENT_LOGGER_URL=http://client-logger:3100
CLIENT_LOGGER_KEY=<openssl rand -hex 24>
```

With no token or chat id every send is a silent no-op. The token is a secret: only the app, the worker and `client-logger` receive it, never the browser or the repository.

Locally, compose already runs `client-logger` and wires the URL; you only add the token and chat id to `.env`. On Railway, see [setup-deploy-railway.md](setup-deploy-railway.md).

## 3. Use it in code

Backend: log through the `*slog.Logger` your service already holds (`app.Log`) and the record is forwarded when it meets the level.

```go
s.log.WarnContext(ctx, "payout delayed", "shop_id", shopID)      // warn
s.log.ErrorContext(ctx, "stripe transfer failed", "err", err)    // error
```

Browser: nothing to do for uncaught errors (`resources/js/lib/clientLog.ts`, installed in `app.tsx`). For an error you catch yourself:

```ts
import { reportClientError } from "./lib/clientLog";

try {
  await riskyCall();
} catch (err) {
  reportClientError("error", String(err), err instanceof Error ? err.stack : undefined);
}
```

Levels are `warn`, `error` and `fatal`. A page sends at most 5 distinct reports, the app accepts 10 per minute per IP, and the service mutes an identical report for 10 minutes.

## 4. Test it

- Go: `go test ./tests/ -run ClientLog` (forwarding, validation) and `go test ./app/alerts/`.
- Browser: in the page console run `setTimeout(() => { throw new Error("telegram check") })` and look at the chat.

## Message limits

Reports are bounded on every hop: message 500 characters, stack 2000, URL 300, user agent 200, body 16 KB. Telegram caps messages at 4096 characters; both libraries trim and HTML-escape.
