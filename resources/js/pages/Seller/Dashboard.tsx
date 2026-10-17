import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import Price from "../../components/catalog/Price";
import SellerNav from "../../components/catalog/SellerNav";
import StatusBadge from "../../components/catalog/StatusBadge";
import { useT } from "../../lib/i18n";
import type { Page, SellerProduct, ShopView } from "../../types";

export default function Dashboard({ shop, products }: { shop: ShopView; products: Page<SellerProduct> }): JSX.Element {
	const { t } = useT();

	return (
		<>
			<Head title={shop.name} />
			<SellerNav current="/seller" shopSlug={shop.slug} />
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold text-ink">{shop.name}</h1>
					<p className="text-sm text-muted">{t("seller.dashboard.summary", { total: products.total })}</p>
				</div>
				<Link
					href="/seller/products/new"
					className="rounded-md bg-action-600 px-4 py-2 font-medium text-white hover:bg-action-700"
				>
					{t("seller.products.new")}
				</Link>
			</div>
			{shop.status !== "active" && (
				<p role="alert" className="mt-4 rounded-md bg-danger-600/10 px-4 py-3 text-sm text-danger-600">
					{t("seller.dashboard.suspended")}
				</p>
			)}
			<section aria-labelledby="recent-products" className="mt-6 rounded-md bg-card shadow-sm">
				<h2 id="recent-products" className="border-b border-line px-4 py-3 font-medium text-ink">
					{t("seller.dashboard.recent")}
				</h2>
				{products.data.length === 0 ? (
					<p className="px-4 py-6 text-sm text-muted">{t("seller.products.empty")}</p>
				) : (
					<ul className="divide-y divide-line">
						{products.data.slice(0, 5).map((p) => (
							<li key={p.id} className="flex items-center gap-3 px-4 py-3">
								<Link
									href={`/seller/products/${p.id}/edit`}
									className="min-w-0 flex-1 truncate text-ink hover:text-action-600"
								>
									{p.title}
								</Link>
								<StatusBadge status={p.status} />
								<Price cents={p.price_cents} size="sm" />
							</li>
						))}
					</ul>
				)}
			</section>
		</>
	);
}
