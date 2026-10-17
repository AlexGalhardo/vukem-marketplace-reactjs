import { z } from "zod";

import { containsContact } from "./moderation";

// Review rules, mirroring app/reviews/input.go and app/moderation (the
// server is the source of truth): 1 to 5 stars, optional title and text,
// lengths counted in code points (Go runes), no links, emails or phones.
export const TITLE_MAX = 80;
export const BODY_MAX = 2000;
export const REPLY_MAX = 1000;

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
