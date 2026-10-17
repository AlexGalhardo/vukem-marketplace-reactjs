import { Head, Link, usePage } from "@inertiajs/react";
import type { JSX } from "react";

import CategoryGrid from "../components/catalog/CategoryGrid";
import HeroCarousel from "../components/catalog/HeroCarousel";
import ProductGrid from "../components/catalog/ProductGrid";
import Shelf from "../components/catalog/Shelf";
import { CardIcon, PinIcon, StoreIcon, TagIcon, UserIcon } from "../components/icons";
import { type MessageKey, useT } from "../lib/i18n";
import type { CategoryRef, ProductCard, SharedProps } from "../types";

interface Props {
	categories: CategoryRef[];
	latest: ProductCard[];
	deals: ProductCard[];
	recent: ProductCard[];
}

interface Shortcut {
	icon: (p: { className?: string }) => JSX.Element;
	title: MessageKey;
	body: MessageKey;
	cta: MessageKey;
	href: string;
}

// Shortcuts are the row of small panels under the banner (account,
// delivery, payment, budget, selling).
function Shortcuts(): JSX.Element {
	const { t } = useT();
	const signedIn = Boolean(usePage<SharedProps>().props.auth?.user);
	const items: Shortcut[] = [
		signedIn
			? {
					icon: UserIcon,
					title: "storefront.shortcuts.account_title",
					body: "storefront.shortcuts.account_body",
					cta: "storefront.shortcuts.account_cta",
					href: "/dashboard",
				}
			: {
					icon: UserIcon,
					title: "storefront.shortcuts.login_title",
					body: "storefront.shortcuts.login_body",
					cta: "storefront.shortcuts.login_cta",
					href: "/login",
				},
		{
			icon: PinIcon,
			title: "storefront.shortcuts.cep_title",
			body: "storefront.shortcuts.cep_body",
			cta: "storefront.shortcuts.cep_cta",
			href: signedIn ? "/account/addresses" : "/register",
		},
		{
			icon: CardIcon,
			title: "storefront.shortcuts.pay_title",
			body: "storefront.shortcuts.pay_body",
			cta: "storefront.shortcuts.pay_cta",
			href: "/search",
		},
		{
			icon: TagIcon,
			title: "storefront.shortcuts.budget_title",
			body: "storefront.shortcuts.budget_body",
			cta: "storefront.shortcuts.budget_cta",
			href: "/search?max=10000&sort=price_asc",
		},
		{
			icon: StoreIcon,
			title: "storefront.shortcuts.sell_title",
			body: "storefront.shortcuts.sell_body",
			cta: "storefront.shortcuts.sell_cta",
			href: "/sell",
		},
	];

	return (
		<ul className="relative -mx-4 mt-4 flex snap-x gap-3 overflow-x-auto px-4 pb-1 scrollbar-none md:mx-0 md:grid md:grid-cols-5 md:overflow-visible md:px-0">
			{items.map((s) => (
				<li
					key={s.title}
					className="flex w-44 shrink-0 snap-start flex-col items-center gap-2 rounded-md bg-card p-4 text-center shadow-card md:w-auto"
				>
					<p className="text-sm font-semibold text-ink">{t(s.title)}</p>
					<span className="flex size-14 items-center justify-center rounded-full bg-action-soft text-action-600">
						<s.icon className="size-7" />
					</span>
					<p className="text-xs text-muted">{t(s.body)}</p>
					<Link
						href={s.href}
						className="mt-auto inline-flex h-8 items-center rounded-md bg-action-500 px-3 text-xs font-semibold text-on-action hover:bg-action-600"
					>
						{t(s.cta)}
					</Link>
				</li>
			))}
		</ul>
	);
}

export default function Home({ categories, latest, deals, recent }: Props): JSX.Element {
	const { t } = useT();

	return (
		<>
			<Head title={t("app.name")} />
			<h1 className="sr-only">{t("home.hero_title")}</h1>
			<HeroCarousel />
			<Shortcuts />

			<Shelf id="recent" title={t("catalog.home.recent")} products={recent} />
			<Shelf id="deals" title={t("catalog.home.deals")} products={deals} seeAll="/search?sort=price_asc" />

			<CategoryGrid categories={categories} label={t("nav.categories")} title={t("nav.categories")} />

			<section aria-labelledby="latest" className="mt-6">
				<div className="mb-3 flex items-baseline gap-4">
					<h2 id="latest" className="text-xl font-semibold text-ink sm:text-2xl">
						{t("catalog.home.latest")}
					</h2>
					<Link href="/search?sort=newest" className="text-sm text-action-600 hover:underline">
						{t("catalog.home.see_all")}
					</Link>
				</div>
				<ProductGrid
					products={latest}
					empty={<p className="rounded-md bg-card p-6 text-muted shadow-card">{t("catalog.home.empty")}</p>}
				/>
			</section>
		</>
	);
}
