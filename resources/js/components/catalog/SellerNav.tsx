import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import { type MessageKey, useT } from "../../lib/i18n";

const tabs: { href: string; label: MessageKey }[] = [
	{ href: "/seller", label: "seller.nav.overview" },
	{ href: "/seller/products", label: "seller.nav.products" },
	{ href: "/seller/questions", label: "qna.seller.nav" },
	{ href: "/seller/messages", label: "chat.seller_nav" },
	{ href: "/seller/settings", label: "seller.nav.settings" },
];

// SellerNav is the seller area's section navigation.
export default function SellerNav({ current, shopSlug }: { current: string; shopSlug?: string }): JSX.Element {
	const { t } = useT();

	return (
		<nav aria-label={t("seller.nav.label")} className="mb-5 flex flex-wrap items-center gap-1 border-b border-line">
			{tabs.map((tab) => (
				<Link
					key={tab.href}
					href={tab.href}
					aria-current={tab.href === current ? "page" : undefined}
					className="-mb-px border-b-2 border-transparent px-3 py-2 text-sm text-muted hover:text-ink aria-[current=page]:border-action-600 aria-[current=page]:text-ink"
				>
					{t(tab.label)}
				</Link>
			))}
			{shopSlug && (
				<Link href={`/s/${shopSlug}`} className="ml-auto px-3 py-2 text-sm text-action-600 hover:underline">
					{t("seller.nav.storefront")}
				</Link>
			)}
		</nav>
	);
}
