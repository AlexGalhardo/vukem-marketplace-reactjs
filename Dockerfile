# syntax=docker/dockerfile:1

# 1. Frontend assets (Vite + Tailwind) with Bun.
FROM oven/bun:1.4.4-alpine AS assets
WORKDIR /src
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY tsconfig.json vite.config.ts ./
COPY resources ./resources
RUN bun run build

# 2. Static Go binary; views, translations and the Vite manifest are embedded.
FROM golang:1.27.3-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=assets /src/resources/public/build ./resources/public/build
ARG APP_VERSION=dev
RUN go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

# 3. Runtime: small, non-root.
FROM alpine:3.24.2
RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S app && adduser -S -G app -h /app app
WORKDIR /app
COPY --from=build /out/app /app/app
USER app
ARG APP_VERSION=dev
ENV APP_ENV=production HOST=0.0.0.0 PORT=3000 APP_VERSION=${APP_VERSION}
# 3000 HTTP (Inertia, REST, GraphQL), 50051 gRPC (Railway TCP proxy).
EXPOSE 3000 50051
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s CMD wget -qO- http://127.0.0.1:3000/health >/dev/null || exit 1
# Migrations and reference data are idempotent, so every start applies them.
CMD ["sh", "-c", "/app/app migrate && /app/app reference:sync && exec /app/app"]
