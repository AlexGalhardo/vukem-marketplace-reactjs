import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import type { ProductCard as Card } from "../../types";
import Installments from "./Installments";
import Price from "./Price";
import ProductImage from "./ProductImage";
import Rating from "./Rating";

export type CardLayout = "grid" | "row";

// ProductCard is one product tile: photo, title, rating, price with cents,
// installments and the shipping/stock line. The whole tile is the link.
// "row" is the wide search-results variant.
export default function ProductCard({ product, layout = "grid" }: { product: Card; layout?: CardLayout }): JSX.Element {
	const { t } = useT();
	const row = layout === "row";
	const lastUnits = product.stock > 0 && product.stock <= 3;

	return (
		<Link
			href={`/p/${product.slug}`}
			className={`group flex h-full overflow-hidden rounded-md bg-card shadow-card transition-shadow duration-200 hover:shadow-lift ${
				row ? "flex-row" : "flex-col"
			}`}
		>
			<ProductImage
				src={product.image_url}
				alt=""
				className={row ? "w-32 shrink-0 p-2 sm:w-44 sm:p-3" : "border-b border-line p-3"}
			/>
			<div className={`flex min-w-0 flex-1 flex-col gap-1 ${row ? "p-3 sm:p-4" : "p-3.5"}`}>
				<h3
					className={`line-clamp-2 break-words text-ink group-hover:text-action-600 ${row ? "text-base sm:text-lg" : "text-sm"}`}
				>
					{product.title}
				</h3>
				<Rating avgX100={product.rating_avg_x100 ?? 0} count={product.rating_count ?? 0} />
				<div className="mt-1">
					<Price cents={product.price_cents} size={row ? "md" : "sm"} />
					<Installments cents={product.price_cents} className="mt-0.5" />
				</div>
				<p className={`mt-auto pt-1.5 text-xs ${lastUnits ? "font-medium text-warning-600" : "text-muted"}`}>
					{lastUnits
						? t("catalog.card.last_units", { count: product.stock })
						: t("catalog.card.ships_from", { uf: product.ship_from_uf })}
				</p>
			</div>
		</Link>
	);
}
