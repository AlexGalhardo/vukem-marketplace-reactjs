import { Link } from "@inertiajs/react";
import { type JSX, useRef } from "react";

import { useT } from "../../lib/i18n";
import type { ProductCard as Card } from "../../types";
import { ChevronLeftIcon, ChevronRightIcon } from "../icons";
import ProductCard from "./ProductCard";

// Shelf is a titled, horizontally scrolling row of product cards inside a
// white panel, with arrow buttons on wider screens and swipe on phones.
export default function Shelf({
	id,
	title,
	products,
	seeAll,
}: {
	id: string;
	title: string;
	products: Card[];
	seeAll?: string;
}): JSX.Element | null {
	const { t } = useT();
	const track = useRef<HTMLUListElement>(null);
	if (products.length === 0) return null;

	const scroll = (dir: 1 | -1): void => {
		const el = track.current;
		if (!el) return;
		const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
		el.scrollBy({ left: dir * el.clientWidth * 0.9, behavior: reduce ? "auto" : "smooth" });
	};
	const arrow =
		"absolute top-1/2 hidden size-11 -translate-y-1/2 items-center justify-center rounded-full bg-card text-action-600 shadow-lift hover:text-action-700 md:flex";

	return (
		<section aria-labelledby={id} className="mt-6 rounded-md bg-card p-4 shadow-card sm:p-5">
			<div className="mb-3 flex items-baseline gap-4">
				<h2 id={id} className="text-xl font-semibold text-ink sm:text-2xl">
					{title}
				</h2>
				{seeAll && (
					<Link href={seeAll} className="text-sm text-action-600 hover:underline">
						{t("catalog.home.see_all")}
					</Link>
				)}
			</div>
			<div className="relative">
				<ul
					ref={track}
					className="relative -mx-1 flex snap-x snap-mandatory gap-3 overflow-x-auto scroll-smooth px-1 py-1 scrollbar-none"
				>
					{products.map((p) => (
						<li key={p.id} className="w-[44%] shrink-0 snap-start sm:w-[30%] md:w-[23%] lg:w-[18.6%]">
							<ProductCard product={p} />
						</li>
					))}
				</ul>
				<button
					type="button"
					onClick={() => scroll(-1)}
					aria-label={t("storefront.shelf.previous", { title })}
					className={`${arrow} -left-6`}
				>
					<ChevronLeftIcon className="size-6" />
				</button>
				<button
					type="button"
					onClick={() => scroll(1)}
					aria-label={t("storefront.shelf.next", { title })}
					className={`${arrow} -right-6`}
				>
					<ChevronRightIcon className="size-6" />
				</button>
			</div>
		</section>
	);
}
