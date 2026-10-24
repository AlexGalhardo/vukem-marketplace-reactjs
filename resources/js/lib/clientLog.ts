// Browser error reporting: uncaught errors and unhandled rejections go to
// POST /client-log, which forwards them to Telegram (the token never
// reaches the browser). Capped per page load so a render loop cannot flood
// the endpoint.
const MAX_REPORTS = 5;

type Level = "warn" | "error" | "fatal";

let sent = 0;
const seen = new Set<string>();

export function reportClientError(level: Level, message: string, stack?: string): void {
	const key = `${level}|${message}`;
	if (sent >= MAX_REPORTS || seen.has(key)) return;
	seen.add(key);
	sent++;

	fetch("/client-log", {
		method: "POST",
		keepalive: true,
		headers: { "Content-Type": "application/json", Accept: "application/json" },
		body: JSON.stringify({
			level,
			message: message.slice(0, 500) || "unknown error",
			stack: stack?.slice(0, 2000),
			url: window.location.pathname.slice(0, 300),
			userAgent: navigator.userAgent.slice(0, 200),
		}),
	}).catch(() => {});
}

function describe(reason: unknown): { message: string; stack?: string } {
	if (reason instanceof Error) return { message: `${reason.name}: ${reason.message}`, stack: reason.stack };
	return { message: String(reason) };
}

export function installClientLogger(): void {
	window.addEventListener("error", (e) => {
		const { message, stack } = describe(e.error ?? e.message);
		reportClientError("error", message, stack);
	});
	window.addEventListener("unhandledrejection", (e) => {
		const { message, stack } = describe(e.reason);
		reportClientError("error", `Unhandled rejection: ${message}`, stack);
	});
}
