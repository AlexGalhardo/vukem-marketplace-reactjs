import { z } from "zod";

import { containsContact } from "./moderation";

// Question rules, mirroring app/qna/input.go and app/moderation. The server is
// the source of truth; this only spares a round trip.
export const QUESTION_MIN = 10;
export const QUESTION_MAX = 500;
export const ANSWER_MAX = 1000;
export const REASON_MIN = 3;
export const REASON_MAX = 500;

// Length in code points, like Go's rune count.
function length(text: string): number {
	return Array.from(text).length;
}

export interface TextMessages {
	tooShort: string;
	tooLong: string;
	contact: string;
}

// textSchema trims the text and checks its length and contact data.
export function textSchema(min: number, max: number, messages: TextMessages): z.ZodType<string, string> {
	return z
		.string()
		.transform((value) => value.trim())
		.refine((value) => length(value) >= min, messages.tooShort)
		.refine((value) => length(value) <= max, messages.tooLong)
		.refine((value) => !containsContact(value), messages.contact);
}

// check returns the first message textSchema reports for value, if any.
export function check(schema: z.ZodType<string, string>, value: string): string | undefined {
	const result = schema.safeParse(value);
	return result.success ? undefined : result.error.issues[0]?.message;
}
