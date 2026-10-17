import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import { type MessageKey, useT } from "../../lib/i18n";
import { CardIcon, ShieldIcon, TruckIcon } from "../icons";

const columns: { title: MessageKey; links: { href: string; label: MessageKey }[] }[] = [
	{
		title: "storefront.footer.buy",
		links: [
			{ href: "/search", label: "storefront.footer.all_products" },
			{ href: "/search?sort=price_asc", label: "storefront.footer.deals" },
			{ href: "/search?sort=newest", label: "storefront.footer.newest" },
			{ href: "/cart", label: "storefront.footer.cart" },
		],
	},
	{
		title: "storefront.footer.sell",
		links: [
			{ href: "/sell", label: "storefront.footer.open_shop" },
			{ href: "/seller", label: "storefront.footer.seller_area" },
			{ href: "/seller/payouts", label: "storefront.footer.payouts" },
		],
	},
	{
		title: "storefront.footer.help",
		links: [
			{ href: "/dashboard", label: "storefront.footer.purchases" },
			{ href: "/account/addresses", label: "storefront.footer.addresses" },
			{ href: "/contact", label: "storefront.footer.contact" },
		],
	},
];

const perks: { icon: (p: { className?: string }) => JSX.Element; title: MessageKey; body: MessageKey }[] = [
	{ icon: CardIcon, title: "storefront.perks.pay_title", body: "storefront.perks.pay_body" },
	{ icon: TruckIcon, title: "storefront.perks.ship_title", body: "storefront.perks.ship_body" },
	{ icon: ShieldIcon, title: "storefront.perks.safe_title", body: "storefront.perks.safe_body" },
];

// SiteFooter: the three service promises, link columns and the legal line
// with the non-affiliation note.
export default function SiteFooter(): JSX.Element {
	const { t } = useT();

	return (
		<footer className="mt-12 bg-card text-ink">
			<ul className="mx-auto grid max-w-6xl gap-6 px-4 py-8 sm:grid-cols-3">
				{perks.map(({ icon: PerkIcon, title, body }) => (
					<li key={title} className="flex flex-col items-center gap-2 text-center">
						<span className="flex size-14 items-center justify-center rounded-full bg-action-soft text-action-600">
							<PerkIcon className="size-7" />
						</span>
						<p className="font-medium">{t(title)}</p>
						<p className="max-w-64 text-sm text-muted">{t(body)}</p>
					</li>
				))}
			</ul>
			<div className="border-t border-line">
				<nav
					aria-label={t("storefront.footer.label")}
					className="mx-auto grid max-w-6xl grid-cols-2 gap-6 px-4 py-8 sm:grid-cols-4"
				>
					{columns.map((col) => (
						<div key={col.title}>
							<h2 className="mb-2 text-sm font-semibold">{t(col.title)}</h2>
							<ul className="space-y-1.5 text-sm">
								{col.links.map((l) => (
									<li key={l.href}>
										<Link href={l.href} className="text-muted hover:text-ink hover:underline">
											{t(l.label)}
										</Link>
									</li>
								))}
							</ul>
						</div>
					))}
					<div>
						<h2 className="mb-2 text-sm font-semibold">{t("storefront.footer.project")}</h2>
						<ul className="space-y-1.5 text-sm">
							<li>
								<a href="https://vukem.dev" className="text-muted hover:text-ink hover:underline">
									{t("storefront.footer.framework")}
								</a>
							</li>
						</ul>
					</div>
				</nav>
			</div>
			<div className="border-t border-line bg-sunken">
				<div className="mx-auto max-w-6xl space-y-1 px-4 py-5 text-xs text-muted">
					<p>{t("footer.rights", { year: new Date().getFullYear() })}</p>
					<p>{t("storefront.footer.disclaimer")}</p>
				</div>
			</div>
		</footer>
	);
}
