import { z } from "zod";

// Review rules, mirroring app/reviews/input.go and app/moderation (the
// server is the source of truth): 1 to 5 stars, optional title and text,
// lengths counted in code points (Go runes), no links, emails or phones.
export const TITLE_MAX = 80;
export const BODY_MAX = 2000;
export const REPLY_MAX = 1000;

const url = /\b(?:https?:\/\/|www\.)\S+/i;
const domain =
	/\b[a-z0-9][a-z0-9-]*\.(?:com|net|org|br|io|me|app|dev|info|biz|co|link|shop|store|site|online|xyz|ly|gl)\b/i;
const email = /[a-z0-9._%+-]+\s*(?:@|\(at\)|\[at\])\s*[a-z0-9-]+(?:\s*(?:\.|\(dot\)|\[dot\])\s*[a-z]{2,})+/i;
const handle = /\b(?:whats\s*app|whatsapp|zap|telegram|wa\.me|t\.me|instagram|insta)\b/i;

function hasPhone(text: string): boolean {
	let digits = 0;
	for (const char of text) {
		if (/\p{Nd}/u.test(char)) {
			digits++;
			if (digits >= 8) return true;
		} else if (!" .-()+".includes(char)) {
			digits = 0;
		}
	}
	return false;
}

export function containsContact(text: string): boolean {
	return url.test(text) || domain.test(text) || email.test(text) || handle.test(text) || hasPhone(text);
}

export const length = (value: string): number => Array.from(value.trim()).length;

export interface ReviewMessages {
	rating: string;
	tooLong: (max: number) => string;
	contact: string;
}

export interface ReviewFields {
	rating: number;
	title: string;
	body: string;
	recommend: "" | "yes" | "no";
}

export function reviewSchema(m: ReviewMessages): z.ZodType<ReviewFields> {
	const text = (max: number): z.ZodType<string> =>
		z
			.string()
			.refine((v) => length(v) <= max, m.tooLong(max))
			.refine((v) => !containsContact(v), m.contact);
	return z.object({
		rating: z.number().int().min(1, m.rating).max(5, m.rating),
		title: text(TITLE_MAX),
		body: text(BODY_MAX),
		recommend: z.enum(["", "yes", "no"]),
	});
}

export function replySchema(m: { required: string; tooLong: string; contact: string }): z.ZodType<string> {
	return z
		.string()
		.refine((v) => length(v) >= 1, m.required)
		.refine((v) => length(v) <= REPLY_MAX, m.tooLong)
		.refine((v) => !containsContact(v), m.contact);
}

// fieldErrors maps the first zod issue of each top-level field.
export function fieldErrors(error: z.ZodError): Record<string, string> {
	const out: Record<string, string> = {};
	for (const issue of error.issues) {
		const key = String(issue.path[0] ?? "");
		if (key && !(key in out)) out[key] = issue.message;
	}
	return out;
}
