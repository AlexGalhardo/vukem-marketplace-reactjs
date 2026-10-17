import type { JSX, ReactNode } from "react";

import type { ProductCard as Card } from "../../types";
import ProductCard from "./ProductCard";

// ProductGrid lays product tiles out responsively; empty shows instead
// when there is nothing to list.
export default function ProductGrid({ products, empty }: { products: Card[]; empty?: ReactNode }): JSX.Element {
	if (products.length === 0) return <>{empty}</>;

	return (
		<ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
			{products.map((p) => (
				<li key={p.id}>
					<ProductCard product={p} />
				</li>
			))}
		</ul>
	);
}
