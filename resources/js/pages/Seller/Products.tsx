import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import Pagination from "../../components/catalog/Pagination";
import ProductImage from "../../components/catalog/ProductImage";
import SellerNav from "../../components/catalog/SellerNav";
import StatusBadge from "../../components/catalog/StatusBadge";
import { useT } from "../../lib/i18n";
import { formatBRL } from "../../lib/money";
import type { Page, SellerProduct } from "../../types";

export default function Products({ products }: { products: Page<SellerProduct> }): JSX.Element {
	const { t, locale } = useT();

	return (
		<>
			<Head title={t("seller.products.title")} />
			<SellerNav current="/seller/products" />
			<div className="mb-4 flex items-center justify-between gap-3">
				<h1 className="text-2xl font-semibold text-ink">{t("seller.products.title")}</h1>
				<Link
					href="/seller/products/new"
					className="rounded-md bg-action-500 px-4 py-2 font-medium text-on-action hover:bg-action-600"
				>
					{t("seller.products.new")}
				</Link>
			</div>
			{products.data.length === 0 ? (
				<p className="rounded-md bg-card px-4 py-8 text-center text-muted shadow-card">
					{t("seller.products.empty")}
				</p>
			) : (
				<div className="overflow-x-auto rounded-md bg-card shadow-card">
					<table className="w-full text-left text-sm">
						<thead className="border-b border-line text-muted">
							<tr>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("seller.products.product")}
								</th>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("seller.products.status")}
								</th>
								<th scope="col" className="px-4 py-2 text-right font-medium">
									{t("seller.products.price")}
								</th>
								<th scope="col" className="px-4 py-2 text-right font-medium">
									{t("seller.products.stock")}
								</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-line">
							{products.data.map((p) => (
								<tr key={p.id}>
									<td className="px-4 py-2">
										<Link
											href={`/seller/products/${p.id}/edit`}
											className="flex items-center gap-3 text-ink hover:text-action-600"
										>
											<ProductImage
												src={p.images[0]?.url_320 ?? ""}
												alt=""
												className="size-12 shrink-0 rounded border border-line"
											/>
											<span className="line-clamp-2">{p.title}</span>
										</Link>
									</td>
									<td className="px-4 py-2">
										<StatusBadge status={p.status} />
									</td>
									<td className="px-4 py-2 text-right tabular-nums">
										{formatBRL(p.price_cents, locale)}
									</td>
									<td className="px-4 py-2 text-right tabular-nums">{p.stock}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
			<Pagination page={products.page} lastPage={products.last_page} />
		</>
	);
}
