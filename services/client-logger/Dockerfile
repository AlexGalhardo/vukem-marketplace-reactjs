# syntax=docker/dockerfile:1
FROM oven/bun:1.4.3-alpine
WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --production
COPY index.ts ./
USER bun
ENV PORT=3100
EXPOSE 3100
HEALTHCHECK --interval=15s --timeout=3s CMD wget -qO- http://127.0.0.1:3100/health >/dev/null || exit 1
CMD ["bun", "index.ts"]
