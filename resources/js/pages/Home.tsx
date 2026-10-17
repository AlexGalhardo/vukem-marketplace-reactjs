import { Head, Link } from "@inertiajs/react";
import type { JSX, ReactNode } from "react";

import CategoryRail from "../components/catalog/CategoryRail";
import ProductGrid from "../components/catalog/ProductGrid";
import { useT } from "../lib/i18n";
import type { CategoryRef, ProductCard } from "../types";

interface Props {
	categories: CategoryRef[];
	latest: ProductCard[];
	deals: ProductCard[];
	recent: ProductCard[];
}

function Section({
	id,
	title,
	action,
	children,
}: {
	id: string;
	title: string;
	action?: ReactNode;
	children: ReactNode;
}) {
	return (
		<section aria-labelledby={id} className="mt-8">
			<div className="mb-3 flex items-baseline justify-between gap-4">
				<h2 id={id} className="text-xl font-semibold text-ink">
					{title}
				</h2>
				{action}
			</div>
			{children}
		</section>
	);
}

export default function Home({ categories, latest, deals, recent }: Props): JSX.Element {
	const { t } = useT();
	const seeAll = (href: string) => (
		<Link href={href} className="text-sm text-action-600 hover:underline">
			{t("catalog.home.see_all")}
		</Link>
	);

	return (
		<>
			<Head title={t("app.name")} />
			<section className="flex flex-col gap-4 rounded-md bg-card px-5 py-6 shadow-sm sm:flex-row sm:items-center sm:justify-between">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight text-ink sm:text-3xl">
						{t("home.hero_title")}
					</h1>
					<p className="mt-1 max-w-xl text-muted">{t("home.hero_body")}</p>
				</div>
				<Link
					href="/sell"
					className="shrink-0 self-start rounded-md bg-action-600 px-5 py-2.5 font-medium text-on-action hover:bg-action-700 sm:self-auto"
				>
					{t("home.cta_sell")}
				</Link>
			</section>

			<div className="mt-6">
				<CategoryRail categories={categories} label={t("nav.categories")} />
			</div>

			{recent.length > 0 && (
				<Section id="recent" title={t("catalog.home.recent")}>
					<ProductGrid products={recent.slice(0, 4)} />
				</Section>
			)}

			<Section id="deals" title={t("catalog.home.deals")} action={seeAll("/search?sort=price_asc")}>
				<ProductGrid products={deals} empty={<p className="text-muted">{t("catalog.home.empty")}</p>} />
			</Section>

			<Section id="latest" title={t("catalog.home.latest")} action={seeAll("/search?sort=newest")}>
				<ProductGrid products={latest} empty={<p className="text-muted">{t("catalog.home.empty")}</p>} />
			</Section>
		</>
	);
}
