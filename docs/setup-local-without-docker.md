# Local setup without Docker for the app

Run the Go server and Vite on your machine with hot reload. The backing services (Postgres, Redis, MongoDB, S3, Mailpit) can still come from Docker, or you can install them natively.

## Requirements

- Go 1.27.3
- Bun 1.4.3
- The Vukem CLI: `go install github.com/AlexGalhardo/vukem/cmd/vukem@latest`
- PostgreSQL 18, Redis 8, MongoDB 8, an S3-compatible store and an SMTP catcher

## Steps

1. Install dependencies.

   ```sh
   cp .env.example .env
   vukem key:generate
   bun install
   go mod download
   ```

2. Start the backing services. Using compose for them only:

   ```sh
   docker compose up -d postgres redis mongo mailpit rustfs
   ```

   Without Docker, start your own instances and point `DATABASE_URL`, `REDIS_URL`, `MONGO_URL`, `S3_*` and `SMTP_*` in `.env` at them. The defaults in `.env.example` match the compose ports.

3. Migrate and load reference data.

   ```sh
   vukem migrate
   vukem reference:sync
   ```

4. Create the bucket and the demo data (development only).

   ```sh
   vukem media:bucket
   vukem db:seed
   ```

5. Run the server with Vite hot reload.

   ```sh
   vukem serve
   ```

   Open <http://localhost:3000>.

6. In other terminals, run the worker and the scheduler if you need emails, thumbnails or PIX expiry.

   ```sh
   vukem queue:work
   vukem schedule:work
   ```

## Browser error logger (optional)

```sh
cd services/client-logger
bun install
CLIENT_LOGGER_KEY=<16+ chars> TELEGRAM_BOT_TOKEN=... TELEGRAM_CHAT_ID=... bun index.ts
```

Then set `CLIENT_LOGGER_URL=http://localhost:3100` and the same `CLIENT_LOGGER_KEY` for the app. Details in [telegram-logging.md](telegram-logging.md).

## Checks

```sh
golangci-lint run ./... && go test -race ./...
bun run lint && bun run typecheck && bun run build
```

Narrow runs while developing: `go test ./tests/catalog/ -run Suggest`, `bunx biome check resources`.
