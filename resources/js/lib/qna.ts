import { z } from "zod";

// Question rules, mirroring app/qna (input.go, contact.go). The server is
// the source of truth; this only spares a round trip.
export const QUESTION_MIN = 10;
export const QUESTION_MAX = 500;
export const ANSWER_MAX = 1000;
export const REASON_MIN = 3;
export const REASON_MAX = 500;

const contactURL = new RegExp(
	String.raw`\b(?:https?|ftp)://|\bwww\d{0,3}\.|` +
		String.raw`\b[a-z0-9][a-z0-9-]*\.(?:com|net|org|br|io|me|ly|gl|app|dev|shop|store|link|site|online|xyz|info|biz|co|tv|gg)\b`,
	"i",
);
const contactEmail = /[a-z0-9._%+-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)*\.[a-z]{2,}|(?:^|[\s(:])@[a-z0-9_][a-z0-9_.]{2,}/i;
const contactObfuscated =
	/\barroba\b|[[(]\s*(?:at|dot|ponto)\s*[\])]|\b(?:ponto|dot)\s*(?:com|net|org|br)\b|\bpontocom\b|\b(?:gmail|hotmail)\b/i;
const digitRun = /\d[\d\s.()+/-]*\d/g;
const landline = /\d{4}[\s.-]\d{4}/;
const MIN_PHONE_DIGITS = 9;

// containsContact reports whether text carries a link, an email, a social
// handle or a phone number (Go: qna.ContainsContact).
export function containsContact(text: string): boolean {
	if (contactURL.test(text) || contactEmail.test(text) || contactObfuscated.test(text)) return true;
	for (const run of text.match(digitRun) ?? []) {
		const digits = run.replace(/\D/g, "").length;
		if (digits >= MIN_PHONE_DIGITS || (digits === 8 && landline.test(run))) return true;
	}
	return false;
}

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
