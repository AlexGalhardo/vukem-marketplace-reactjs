// Credential bounds and password rules, mirroring app/requests/password.go
// (the server is the source of truth; these only drive the form UX).
export const PASSWORD_MIN = 8;
export const PASSWORD_MAX = 32;
export const EMAIL_MIN = 12;
export const EMAIL_MAX = 48;

export type PasswordRule = "length" | "lower" | "upper" | "digit" | "special";

export const PASSWORD_RULES: readonly PasswordRule[] = ["length", "lower", "upper", "digit", "special"];

// passwordChecks tells which rules value already meets.
export function passwordChecks(value: string): Record<PasswordRule, boolean> {
	const length = Array.from(value).length;
	return {
		length: length >= PASSWORD_MIN && length <= PASSWORD_MAX,
		lower: /\p{Ll}/u.test(value),
		upper: /\p{Lu}/u.test(value),
		digit: /\p{Nd}/u.test(value),
		special: /[^\p{L}\p{Nd}]/u.test(value),
	};
}
