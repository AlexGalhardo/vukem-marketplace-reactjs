import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import { useT } from "../../lib/i18n";

// Pagination links to the previous and next pages of the current URL,
// keeping every other query parameter.
export default function Pagination({ page, lastPage }: { page: number; lastPage: number }): JSX.Element | null {
	const { t } = useT();
	if (lastPage <= 1) return null;

	const href = (n: number): string => {
		const url = new URL(window.location.href);
		url.searchParams.set("page", String(n));
		return url.pathname + url.search;
	};
	const linkClass = "rounded-md px-3 py-2 text-sm text-action-600 hover:bg-action-500/10";

	return (
		<nav aria-label={t("catalog.pagination.label")} className="mt-6 flex items-center justify-center gap-2">
			{page > 1 && (
				<Link href={href(page - 1)} className={linkClass} preserveScroll={false}>
					{t("catalog.pagination.previous")}
				</Link>
			)}
			<span className="px-2 text-sm text-muted">{t("catalog.pagination.status", { page, last: lastPage })}</span>
			{page < lastPage && (
				<Link href={href(page + 1)} className={linkClass}>
					{t("catalog.pagination.next")}
				</Link>
			)}
		</nav>
	);
}
