import { z } from "zod";

// Name rules, mirroring app/requests/name.go (the server is the source of
// truth): white space trimmed and collapsed, 4 to 24 characters counted
// in code points (Go runes), first letter upper-cased.
export const NAME_MIN = 4;
export const NAME_MAX = 24;

export function normalizeName(value: string): string {
	return value.trim().split(/\s+/u).filter(Boolean).join(" ");
}

// capitalizeName upper-cases the first non-space character, keeping the
// rest as typed. Characters whose upper case is longer ("ß" → "SS") stay,
// like Go's unicode.ToUpper.
export function capitalizeName(value: string): string {
	const chars = Array.from(value);
	const i = chars.findIndex((c) => !/\s/u.test(c));
	const first = chars[i];
	if (first === undefined) return value;
	const upper = first.toUpperCase();
	if (Array.from(upper).length !== 1) return value;
	chars[i] = upper;
	return chars.join("");
}

export function nameSchema(message: string): z.ZodType<string, string> {
	return z
		.string()
		.transform((value) => capitalizeName(normalizeName(value)))
		.refine((value) => {
			const length = Array.from(value).length;
			return length >= NAME_MIN && length <= NAME_MAX;
		}, message);
}

// parseName checks value with nameSchema: the normalized name, or the
// message to show.
export function parseName(value: string, message: string): { name: string } | { error: string } {
	const result = nameSchema(message).safeParse(value);
	return result.success ? { name: result.data } : { error: result.error.issues[0]?.message ?? message };
}
