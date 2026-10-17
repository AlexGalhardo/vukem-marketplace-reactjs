import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import type { ProductCard as Card } from "../../types";
import Price from "./Price";
import ProductImage from "./ProductImage";

// ProductCard is one tile of a product grid. The whole tile is the link.
export default function ProductCard({ product }: { product: Card }): JSX.Element {
	const { t } = useT();

	return (
		<Link
			href={`/p/${product.slug}`}
			className="group flex h-full flex-col rounded-md bg-card shadow-sm ring-1 ring-line/60 hover:shadow-md"
		>
			<ProductImage src={product.image_url} alt="" className="rounded-t-md border-b border-line/60" />
			<div className="flex flex-1 flex-col gap-1 p-3">
				<Price cents={product.price_cents} size="sm" />
				<h3 className="line-clamp-2 text-sm text-ink group-hover:text-action-600">{product.title}</h3>
				<p className="mt-auto pt-1 text-xs text-muted">
					{product.stock > 0 && product.stock <= 3
						? t("catalog.card.last_units", { count: product.stock })
						: t("catalog.card.ships_from", { uf: product.ship_from_uf })}
				</p>
			</div>
		</Link>
	);
}
