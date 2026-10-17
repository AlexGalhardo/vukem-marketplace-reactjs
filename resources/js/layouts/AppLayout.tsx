import { CSPProvider } from "@base-ui/react/csp-provider";
import { usePage } from "@inertiajs/react";
import { type JSX, type ReactNode, useEffect } from "react";

import SiteFooter from "../components/layout/SiteFooter";
import SiteHeader from "../components/layout/SiteHeader";
import { useT } from "../lib/i18n";
import type { SharedProps } from "../types";

const nonce = document.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]')?.nonce;

export default function AppLayout({ children }: { children: ReactNode }): JSX.Element {
	const page = usePage<SharedProps>();
	const { locale } = useT();

	// The server sets <html lang> on first visits; Inertia visits keep it in sync.
	useEffect(() => {
		document.documentElement.lang = locale;
	}, [locale]);
	const message = page.flash?.success ?? page.flash?.status;

	return (
		<CSPProvider nonce={nonce}>
			<SiteHeader />
			<main id="main" tabIndex={-1} className="mx-auto max-w-6xl px-4 py-6 focus:outline-none">
				{typeof message === "string" && (
					<p role="status" className="mb-4 rounded-md bg-card px-4 py-3 text-ink shadow-card">
						{message}
					</p>
				)}
				{children}
			</main>
			<SiteFooter />
		</CSPProvider>
	);
}
