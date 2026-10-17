import { useEffect, useState } from "react";

// The last CEP a shopper priced shipping for, kept in this browser only
// (never sent anywhere by itself) so the header can show "Deliver to …"
// and the freight box can prefill it.
const key = "cep";
const event = "vukem:cep";

export function formatCep(digits: string): string {
	return digits.length === 8 ? `${digits.slice(0, 5)}-${digits.slice(5)}` : digits;
}

function read(): string {
	try {
		const v = localStorage.getItem(key) ?? "";
		return /^\d{8}$/.test(v) ? v : "";
	} catch {
		return "";
	}
}

export function saveCep(digits: string): void {
	try {
		localStorage.setItem(key, digits);
	} catch {
		// Storage blocked (private mode): the header just keeps asking for a CEP.
	}
	window.dispatchEvent(new Event(event));
}

export function useSavedCep(): string {
	const [cep, setCep] = useState("");
	useEffect(() => {
		const sync = (): void => setCep(read());
		sync();
		window.addEventListener(event, sync);
		window.addEventListener("storage", sync);
		return () => {
			window.removeEventListener(event, sync);
			window.removeEventListener("storage", sync);
		};
	}, []);
	return cep;
}
