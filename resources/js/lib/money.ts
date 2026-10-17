// Money is integer cents (BRL) everywhere; these helpers only format and
// parse for display. The server re-validates every amount.

export function formatBRL(cents: number, locale: string): string {
	return new Intl.NumberFormat(locale, { style: "currency", currency: "BRL" }).format(cents / 100);
}

// priceParts splits a price for the big "R$ 1.299 ⁹⁰" display.
export function priceParts(cents: number, locale: string): { symbol: string; whole: string; fraction: string } {
	const parts = new Intl.NumberFormat(locale, { style: "currency", currency: "BRL" }).formatToParts(cents / 100);
	const pick = (types: string[]): string =>
		parts
			.filter((p) => types.includes(p.type))
			.map((p) => p.value)
			.join("");
	return { symbol: pick(["currency"]), whole: pick(["integer", "group"]), fraction: pick(["fraction"]) };
}

// toCents parses what a seller typed ("1.299,90", "1299.90", "15") into
// cents, or NaN. Integer math on the digits avoids float rounding.
export function toCents(input: string): number {
	const s = input.trim().replace(/\s|R\$/g, "");
	// A separator followed by 1–2 final digits is the decimal one; any other
	// separator groups thousands.
	const m = s.match(/^([\d.,]*?)(?:[.,](\d{1,2}))?$/);
	if (!m || !/\d/.test(s)) return Number.NaN;
	const whole = (m[1] ?? "").replace(/[.,]/g, "");
	if (!/^\d*$/.test(whole)) return Number.NaN;
	const frac = (m[2] ?? "").padEnd(2, "0");
	return Number(whole || "0") * 100 + Number(frac);
}

export function centsToInput(cents: number): string {
	return (cents / 100).toFixed(2).replace(".", ",");
}
