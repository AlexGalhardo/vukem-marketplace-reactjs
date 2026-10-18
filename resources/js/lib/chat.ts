import { textSchema } from "./qna";

// Chat rules and wire names, mirroring app/chat (input.go, chat.go). The
// server is the source of truth; the schema only spares a round trip.
export const MESSAGE_MAX = 2000;

export const EVENT_MESSAGE = "message.created";
export const EVENT_READ = "message.read";
export const EVENT_TYPING = "typing";
export const EVENT_INBOX = "inbox.updated";

export const inboxChannel = (userId: number): string => `private-inbox.${userId}`;

export interface MessagesText {
	empty: string;
	tooLong: string;
	contact: string;
}

export const messageSchema = (m: MessagesText): ReturnType<typeof textSchema> =>
	textSchema(1, MESSAGE_MAX, { tooShort: m.empty, tooLong: m.tooLong, contact: m.contact });

// mergeMessages adds incoming messages, dropping duplicates (an event and
// a catch-up can carry the same one) and keeping id order.
export function mergeMessages<T extends { id: number }>(current: T[], incoming: T[]): T[] {
	if (incoming.length === 0) return current;
	const byId = new Map(current.map((m) => [m.id, m]));
	for (const m of incoming) byId.set(m.id, m);
	return [...byId.values()].sort((a, b) => a.id - b.id);
}

// postJSON sends a same-origin JSON request (CSRF: the browser's Origin and
// Sec-Fetch-Site headers) and returns the status and decoded body.
export async function postJSON<T>(url: string, body?: unknown): Promise<{ status: number; data: T | null }> {
	const res = await fetch(url, {
		method: "POST",
		credentials: "same-origin",
		headers: { Accept: "application/json", "Content-Type": "application/json" },
		body: body === undefined ? undefined : JSON.stringify(body),
	});
	let data: T | null = null;
	try {
		data = (await res.json()) as T;
	} catch {
		// 204 or a non-JSON error page.
	}
	return { status: res.status, data };
}
