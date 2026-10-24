import { timingSafeEqual } from "node:crypto";
import { createTelegramLogger } from "@galhardoalex/telegram-logger";

// Browser errors reach Telegram through here: the Go app validates and
// forwards them (POST /client-log), so the bot token never leaves the
// server. Same TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID as the Go backend.
const key = process.env.CLIENT_LOGGER_KEY ?? "";
if (key.length < 16) throw new Error("CLIENT_LOGGER_KEY must have at least 16 characters");

const logger = createTelegramLogger({
	appName: `${process.env.APP_NAME ?? "vukem-marketplace"} (browser)`,
	environment: process.env.APP_ENV ?? "development",
});

const LEVELS = ["warn", "error", "fatal"] as const;
type Level = (typeof LEVELS)[number];

interface Payload {
	level: Level;
	message: string;
	stack?: string;
	url?: string;
	userAgent?: string;
}

const MUTE_MS = 10 * 60_000;
const muted = new Map<string, number>();

function authorized(header: string | null): boolean {
	const got = Buffer.from(header ?? "");
	const want = Buffer.from(key);
	return got.length === want.length && timingSafeEqual(got, want);
}

function parse(raw: unknown): Payload | null {
	if (typeof raw !== "object" || raw === null) return null;
	const p = raw as Record<string, unknown>;
	if (!LEVELS.includes(p.level as Level) || typeof p.message !== "string" || p.message === "") return null;
	const text = (v: unknown, max: number): string | undefined =>
		typeof v === "string" && v !== "" ? v.slice(0, max) : undefined;
	return {
		level: p.level as Level,
		message: p.message.slice(0, 500),
		stack: text(p.stack, 2000),
		url: text(p.url, 300),
		userAgent: text(p.userAgent, 200),
	};
}

// shouldSend mutes a repeated failure for MUTE_MS, so one broken page does
// not flood the chat.
function shouldSend(p: Payload, now: number): boolean {
	for (const [k, until] of muted) if (until <= now) muted.delete(k);
	const fingerprint = `${p.level}|${p.message}|${p.url ?? ""}`;
	if (muted.has(fingerprint)) return false;
	if (muted.size >= 500) return false;
	muted.set(fingerprint, now + MUTE_MS);
	return true;
}

const server = Bun.serve({
	port: Number(process.env.PORT ?? 3100),
	maxRequestBodySize: 16 * 1024,
	async fetch(req) {
		const { pathname } = new URL(req.url);
		if (req.method === "GET" && pathname === "/health") return new Response("ok");
		if (req.method !== "POST" || pathname !== "/log") return new Response(null, { status: 404 });
		if (!authorized(req.headers.get("x-client-logger-key"))) return new Response(null, { status: 401 });

		const payload = parse(await req.json().catch(() => null));
		if (!payload) return new Response(null, { status: 422 });
		if (!shouldSend(payload, Date.now())) return new Response(null, { status: 204 });

		const meta = { url: payload.url, userAgent: payload.userAgent };
		const err = Object.assign(new Error(payload.message), { stack: payload.stack ?? payload.message });
		if (payload.level === "warn") await logger.warn(payload.message, meta);
		else if (payload.level === "fatal") await logger.fatal(err, meta);
		else await logger.error(err, meta);
		return new Response(null, { status: 204 });
	},
});

console.log(`client-logger listening on :${server.port}`);
