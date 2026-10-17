import { usePage } from "@inertiajs/react";

import en from "../../lang/en.json";
import ptBR from "../../lang/pt-BR.json";
import type { SharedProps } from "../types";

// The same catalogs the Go side loads (vukem i18n), so server and client
// strings never drift. A Go test keeps both files' keys in sync.
type Catalog = typeof en;

type Leaves<T, P extends string = ""> = {
	[K in keyof T & string]: T[K] extends string ? `${P}${K}` : Leaves<T[K], `${P}${K}.`>;
}[keyof T & string];

export type MessageKey = Leaves<Catalog>;
export type MessageArgs = Record<string, string | number>;

const catalogs: Record<string, Catalog> = { en, "pt-BR": ptBR };

function lookup(catalog: Catalog, key: string): string | undefined {
	let node: unknown = catalog;
	for (const part of key.split(".")) {
		if (typeof node !== "object" || node === null) return undefined;
		node = (node as Record<string, unknown>)[part];
	}
	return typeof node === "string" ? node : undefined;
}

export function translate(locale: string, key: MessageKey, args: MessageArgs = {}): string {
	const message = lookup(catalogs[locale] ?? en, key) ?? lookup(en, key) ?? key;
	return message.replace(/\{(\w+)\}/g, (match, name: string) => (name in args ? String(args[name]) : match));
}

export function useT(): { t: (key: MessageKey, args?: MessageArgs) => string; locale: string } {
	const { locale } = usePage<SharedProps>().props;
	return { t: (key, args) => translate(locale, key, args), locale };
}
