// Contact rules, mirroring app/moderation (the server is the source of
// truth): links, emails, social handles and phone numbers are refused in
// questions, answers, reviews and seller replies.
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
// handle or a phone number (Go: moderation.ContainsContact).
export function containsContact(text: string): boolean {
	if (contactURL.test(text) || contactEmail.test(text) || contactObfuscated.test(text)) return true;
	for (const run of text.match(digitRun) ?? []) {
		const digits = run.replace(/\D/g, "").length;
		if (digits >= MIN_PHONE_DIGITS || (digits === 8 && landline.test(run))) return true;
	}
	return false;
}
