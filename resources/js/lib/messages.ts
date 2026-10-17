import { type MessageKey, translate } from "./i18n";

// Server validation messages for the commerce forms are catalog keys
// ("cart.errors.out_of_stock"); anything else is shown as sent.
export function serverMessage(locale: string, message: string | undefined): string | undefined {
	if (!message) return undefined;
	return translate(locale, message as MessageKey);
}
