// Money arrives from the server as integer cents (BRL); it is only
// formatted here, never computed.
export function formatMoney(cents: number, locale: string): string {
	return new Intl.NumberFormat(locale, { style: "currency", currency: "BRL" }).format(cents / 100);
}

export function formatDateTime(iso: string, locale: string): string {
	return new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short" }).format(new Date(iso));
}
