import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import { link } from "../../components/ui";
import { useT } from "../../lib/i18n";
import type { ChatMessage, ConversationDetail } from "../../types";

interface Props {
	conversation: ConversationDetail;
	messages: ChatMessage[];
	buyer_id: number;
}

// Conversation shows one conversation to an admin, read-only.
export default function Conversation({ conversation, messages, buyer_id }: Props): JSX.Element {
	const { t, locale } = useT();
	const time = new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short" });
	const title = t("chat.admin.between", { buyer: conversation.buyer, shop: conversation.shop.name });

	return (
		<>
			<Head title={title} />
			<Link href="/admin/messages" className={`${link} text-sm`}>
				{t("chat.admin.back")}
			</Link>
			<h1 className="mt-3 text-2xl font-semibold text-ink">{title}</h1>
			{conversation.product && (
				<p className="text-sm text-muted">{t("chat.show.about", { product: conversation.product.title })}</p>
			)}
			<p role="note" className="mt-3 rounded-md bg-action-soft px-4 py-2 text-sm text-ink">
				{t("chat.admin.read_only")}
			</p>

			<ol className="mt-4 flex flex-col gap-3 rounded-md bg-card p-4 shadow-card">
				{messages.map((m) => {
					const fromBuyer = m.sender_id === buyer_id;
					return (
						<li key={m.id} className="border-l-2 border-line pl-3">
							<p className="text-xs text-muted">
								{fromBuyer ? t("chat.admin.buyer") : t("chat.admin.seller")}
								{", "}
								<time dateTime={m.created_at}>{time.format(new Date(m.created_at))}</time>
							</p>
							<p className="text-sm break-words whitespace-pre-wrap text-ink">{m.body}</p>
						</li>
					);
				})}
			</ol>
		</>
	);
}
