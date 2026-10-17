import type { JSX, ReactNode } from "react";

import type { ProductCard as Card } from "../../types";
import ProductCard, { type CardLayout } from "./ProductCard";

// ProductGrid lays product tiles out responsively (or as a list of wide
// rows); empty shows instead when there is nothing to list.
export default function ProductGrid({
	products,
	empty,
	layout = "grid",
	columns = "default",
}: {
	products: Card[];
	empty?: ReactNode;
	layout?: CardLayout;
	columns?: "default" | "narrow";
}): JSX.Element {
	if (products.length === 0) return <>{empty}</>;

	const grid =
		layout === "row"
			? "flex flex-col gap-2"
			: columns === "narrow"
				? "grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3"
				: "grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-4 xl:grid-cols-5";

	return (
		<ul className={grid}>
			{products.map((p) => (
				<li key={p.id}>
					<ProductCard product={p} layout={layout} />
				</li>
			))}
		</ul>
	);
}
