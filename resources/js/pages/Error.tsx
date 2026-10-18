import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import { type MessageKey, useT } from "../lib/i18n";

interface Props {
	status: number;
	// title is the English status text; the page shows its own translation.
	title: string;
	message: string;
}

const titles: Record<number, MessageKey> = {
	403: "errors.forbidden",
	404: "errors.not_found",
	429: "errors.too_many",
};

// Error is the page for 4xx/5xx answers to browser and Inertia visits
// (vukem's Inertia error page hook, routes.errorPage); 5xx never carry
// details.
export default function ErrorPage({ status, title: statusText, message }: Props): JSX.Element {
	const { t } = useT();
	const title = t(titles[status] ?? (status >= 500 ? "errors.server" : "errors.generic"));

	return (
		<>
			<Head title={`${status} · ${title}`} />
			<section className="mx-auto max-w-xl rounded-md bg-card px-6 py-10 text-center shadow-card">
				<p className="text-5xl font-semibold text-muted">{status}</p>
				<h1 className="mt-3 text-2xl font-semibold text-ink">{title}</h1>
				{message && message !== title && message !== statusText && <p className="mt-2 text-muted">{message}</p>}
				<Link
					href="/"
					className="mt-6 inline-flex h-10 items-center rounded-md bg-action-600 px-5 text-sm font-medium text-on-action hover:bg-action-700"
				>
					{t("errors.home")}
				</Link>
			</section>
		</>
	);
}
