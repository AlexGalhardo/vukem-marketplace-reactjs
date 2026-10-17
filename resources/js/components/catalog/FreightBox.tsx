import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import type { ProductDetail } from "../../types";

// FreightBox is the shipping slot of the product page.
//
// Checkout stream: replace this body with the CEP input and the freight
// quote (GET /api/v1/freight?cep&product). The props stay the same; the
// product carries weight_grams and shop.ship_from_uf.
export default function FreightBox({ product }: { product: ProductDetail }): JSX.Element {
	const { t } = useT();

	return (
		<section aria-labelledby="freight-title" className="rounded-md border border-line p-3 text-sm">
			<h2 id="freight-title" className="font-medium text-ink">
				{t("catalog.product.freight_title")}
			</h2>
			<p className="mt-1 text-muted">
				{t("catalog.product.freight_placeholder", { uf: product.shop.ship_from_uf })}
			</p>
		</section>
	);
}
