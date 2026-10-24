# Local setup with Docker

The fastest way to get the whole stack: app, worker, scheduler, Postgres, Redis, MongoDB, S3 (RustFS), Mailpit and the browser error logger.

## Requirements

- Docker with Compose
- Bun 1.4.3 (only for the JS tooling and tests)
- The Vukem CLI, to generate the app key: `go install github.com/AlexGalhardo/vukem/cmd/vukem@latest` (any release works for `key:generate`)

## Steps

1. Copy the environment file.

   ```sh
   cp .env.example .env
   ```

2. Generate the app key; it is written to `.env` as `APP_KEY`.

   ```sh
   vukem key:generate
   ```

3. Start everything.

   ```sh
   docker compose up -d --build
   ```

4. Wait until the app is ready.

   ```sh
   curl http://localhost:3200/ready
   ```

5. Open <http://localhost:3200>. The `seed` service runs on its own: it creates the bucket and the demo data (see [demo-data-and-images.md](demo-data-and-images.md)).

## Demo accounts

`buyer@example.com`, `seller@example.com` and `admin@example.com`, all with the password in `SEED_PASSWORD` (default `marketplace-demo-password`). The admin's TOTP secret is printed once in `docker compose logs seed`. Production never seeds these accounts.

## Useful URLs

| What | URL |
| --- | --- |
| App | <http://localhost:3200> |
| Mailpit (emails) | <http://localhost:8035> |
| RustFS console | <http://localhost:9201> |
| Postgres / Redis / Mongo | `localhost:5442` / `6389` / `27027` |

## Everyday commands

```sh
docker compose logs -f app          # follow the app logs
docker compose exec -T app /app/app db:seed   # seed again (Git Bash: prefix MSYS_NO_PATHCONV=1)
docker compose down -v              # stop and delete the data
```

## Telegram (optional)

Put `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` in `.env` and restart; see [telegram-logging.md](telegram-logging.md).

## Troubleshooting

- `APP_KEY is missing`: run `vukem key:generate` first.
- Port already in use: set the `MKT_*_PORT` variables in `.env`.
- Images look broken after a rebuild: run `db:seed` again; demo images are embedded in the binary, so a rebuilt image already serves them.
