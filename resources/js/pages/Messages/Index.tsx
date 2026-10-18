import { Head, Link, router } from "@inertiajs/react";
import { type JSX, useMemo, useState } from "react";

import SellerNav from "../../components/catalog/SellerNav";
import { ChatIcon } from "../../components/icons";
import { link } from "../../components/ui";
import { EVENT_INBOX, inboxChannel } from "../../lib/chat";
import { useT } from "../../lib/i18n";
import { type BroadcastHandlers, useBroadcast } from "../../lib/useBroadcast";
import type { ChatRole, ConversationSummary, Page } from "../../types";

interface Props {
	role: ChatRole;
	conversations: Page<ConversationSummary>;
	me: number;
}

// Index is a participant's inbox: the buyer's conversations (/messages) or
// the shop's (/seller/messages). New messages reload the list in place.
export default function Index({ role, conversations, me }: Props): JSX.Element {
	const { t, locale } = useT();
	const [live, setLive] = useState("");
	const seller = role === "seller";
	const base = seller ? "/seller/messages" : "/messages";
	const title = seller ? t("chat.inbox.title_seller") : t("chat.inbox.title_buyer");
	const when = new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short" });

	const handlers = useMemo<BroadcastHandlers>(
		() => ({
			onEvent: (_channel, event) => {
				if (event !== EVENT_INBOX) return;
				router.reload({ only: ["conversations", "chat"] });
				setLive(t("chat.inbox.live"));
			},
		}),
		[t],
	);
	useBroadcast([inboxChannel(me)], [EVENT_INBOX], handlers);

	return (
		<>
			<Head title={title} />
			{seller && <SellerNav current="/seller/messages" />}
			<h1 className="text-2xl font-semibold text-ink">{title}</h1>
			<p className="mt-1 max-w-prose text-sm text-muted">
				{seller ? t("chat.inbox.intro_seller") : t("chat.inbox.intro_buyer")}
			</p>
			<p role="status" className="sr-only">
				{live}
			</p>

			{conversations.data.length === 0 ? (
				<p className="mt-6 flex items-center gap-3 rounded-md bg-card px-4 py-6 text-sm text-muted shadow-card">
					<ChatIcon className="size-6 shrink-0" />
					{seller ? t("chat.inbox.empty_seller") : t("chat.inbox.empty_buyer")}
				</p>
			) : (
				<ul
					aria-label={t("chat.inbox.list_label")}
					className="mt-6 divide-y divide-line rounded-md bg-card shadow-card"
				>
					{conversations.data.map((c) => (
						<li key={c.id}>
							<Link
								href={`/messages/${c.id}`}
								className="flex items-start gap-3 px-4 py-3.5 hover:bg-sunken focus-visible:bg-sunken"
							>
								<span
									aria-hidden="true"
									className="flex size-10 shrink-0 items-center justify-center rounded-full bg-action-soft font-semibold text-action-600"
								>
									{c.with.slice(0, 1).toUpperCase()}
								</span>
								<span className="min-w-0 flex-1">
									<span className="flex items-baseline justify-between gap-3">
										<span
											className={`truncate text-ink ${c.unread > 0 ? "font-semibold" : "font-medium"}`}
										>
											{c.with}
										</span>
										{c.last_message_at && (
											<time
												dateTime={c.last_message_at}
												className="shrink-0 text-xs text-muted tabular-nums"
											>
												{when.format(new Date(c.last_message_at))}
											</time>
										)}
									</span>
									<span className="block truncate text-xs text-muted">
										{c.product
											? t("chat.inbox.about", { product: c.product.title })
											: t("chat.inbox.general")}
									</span>
									<span className="mt-0.5 flex items-center justify-between gap-3">
										<span
											className={`truncate text-sm ${c.unread > 0 ? "text-ink" : "text-muted"}`}
										>
											{c.last_from_me
												? t("chat.inbox.you", { text: c.last_message })
												: c.last_message}
										</span>
										{c.unread > 0 && (
											<span className="shrink-0 rounded-full bg-action-500 px-2 py-0.5 text-xs font-semibold text-on-action tabular-nums">
												{t("chat.inbox.unread", { count: c.unread })}
											</span>
										)}
									</span>
								</span>
							</Link>
						</li>
					))}
				</ul>
			)}

			{conversations.last_page > 1 && (
				<nav aria-label={t("chat.inbox.pagination")} className="mt-6 flex items-center justify-between text-sm">
					{conversations.page > 1 ? (
						<Link href={`${base}?page=${conversations.page - 1}`} className={link}>
							{t("chat.inbox.previous")}
						</Link>
					) : (
						<span />
					)}
					<span className="text-muted">
						{t("chat.inbox.page_of", { page: conversations.page, last: conversations.last_page })}
					</span>
					{conversations.page < conversations.last_page ? (
						<Link href={`${base}?page=${conversations.page + 1}`} className={link}>
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
