import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import { link } from "../../components/ui";
import { useT } from "../../lib/i18n";
import type { ConversationSummary, Page } from "../../types";

// Messages lists every buyer ↔ seller conversation for moderation; it is
// read-only and opening one is audited.
export default function Messages({ conversations }: { conversations: Page<ConversationSummary> }): JSX.Element {
	const { t, locale } = useT();
	const when = new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short" });

	return (
		<>
			<Head title={t("chat.admin.title")} />
			<h1 className="text-2xl font-semibold text-ink">{t("chat.admin.title")}</h1>
			<p className="mt-1 max-w-prose text-sm text-muted">{t("chat.admin.intro")}</p>

			{conversations.data.length === 0 ? (
				<p className="mt-6 rounded-md bg-card px-4 py-6 text-sm text-muted shadow-card">
					{t("chat.admin.empty")}
				</p>
			) : (
				<ul className="mt-6 divide-y divide-line rounded-md bg-card shadow-card">
					{conversations.data.map((c) => (
						<li key={c.id} className="px-4 py-3">
							<Link href={`/admin/messages/${c.id}`} className={`${link} font-medium`}>
								{t("chat.admin.between", { buyer: c.with, shop: c.shop.name })}
							</Link>
							<p className="truncate text-sm text-muted">{c.last_message}</p>
							{c.last_message_at && (
								<time dateTime={c.last_message_at} className="text-xs text-muted tabular-nums">
									{when.format(new Date(c.last_message_at))}
								</time>
							)}
						</li>
					))}
				</ul>
			)}

			{conversations.last_page > 1 && (
				<nav aria-label={t("chat.inbox.pagination")} className="mt-6 flex items-center justify-between text-sm">
					{conversations.page > 1 ? (
						<Link href={`/admin/messages?page=${conversations.page - 1}`} className={link}>
							{t("chat.inbox.previous")}
						</Link>
					) : (
						<span />
					)}
					<span className="text-muted">
						{t("chat.inbox.page_of", { page: conversations.page, last: conversations.last_page })}
					</span>
					{conversations.page < conversations.last_page ? (
						<Link href={`/admin/messages?page=${conversations.page + 1}`} className={link}>
							{t("chat.inbox.next")}
						</Link>
					) : (
						<span />
					)}
				</nav>
			)}
		</>
	);
}
