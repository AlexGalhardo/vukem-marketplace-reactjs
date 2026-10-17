import { Head } from "@inertiajs/react";
import type { JSX } from "react";

import Pagination from "../../components/catalog/Pagination";
import ProductGrid from "../../components/catalog/ProductGrid";
import { useT } from "../../lib/i18n";
import type { Page, ProductCard, ShopView } from "../../types";

interface Props {
	shop: ShopView;
	results: Page<ProductCard>;
}

export default function Shop({ shop, results }: Props): JSX.Element {
	const { t, locale } = useT();
	const since = new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" }).format(new Date(shop.since));

	return (
		<>
			<Head title={shop.name} />
			<header className="overflow-hidden rounded-md bg-card shadow-sm">
				<div className="h-20 bg-brand-300 dark:bg-brand-900" />
				<div className="flex flex-col gap-3 px-5 pb-5 sm:flex-row sm:items-end">
					<div className="-mt-10 size-20 shrink-0 overflow-hidden rounded-full border-4 border-card bg-white">
						{shop.logo_url ? (
							<img src={shop.logo_url} alt="" className="size-full object-cover" />
						) : (
							<span
								aria-hidden="true"
								className="flex size-full items-center justify-center text-2xl font-semibold text-brand-900"
							>
								{shop.name.charAt(0).toUpperCase()}
							</span>
						)}
					</div>
					<div className="min-w-0">
						<h1 className="text-2xl font-semibold text-ink">{shop.name}</h1>
						<p className="text-sm text-muted">
							{t("shop.meta", { since, uf: shop.ship_from_uf, count: results.total })}
						</p>
					</div>
				</div>
				{shop.description && <p className="max-w-prose px-5 pb-5 text-sm text-muted">{shop.description}</p>}
			</header>
			<section aria-labelledby="shop-products" className="mt-6">
				<h2 id="shop-products" className="mb-3 text-xl font-semibold text-ink">
					{t("shop.products")}
				</h2>
				<ProductGrid products={results.data} empty={<p className="text-muted">{t("shop.empty")}</p>} />
				<Pagination page={results.page} lastPage={results.last_page} />
			</section>
		</>
	);
}
