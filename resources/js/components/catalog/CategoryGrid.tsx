import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import type { CategoryRef } from "../../types";
import { CategoryIcon } from "../icons";

// CategoryGrid is the home page's category panel: one tile per category
// with its own line glyph.
export default function CategoryGrid({
	categories,
	label,
	title,
}: {
	categories: CategoryRef[];
	label: string;
	title: string;
}): JSX.Element | null {
	if (categories.length === 0) return null;

	return (
		<nav aria-label={label} className="mt-6 rounded-md bg-card p-4 shadow-card sm:p-5">
			<h2 className="mb-3 text-xl font-semibold text-ink sm:text-2xl">{title}</h2>
			<ul className="grid grid-cols-3 border-t border-l border-line sm:grid-cols-4 lg:grid-cols-6">
				{categories.map((c) => (
					<li key={c.slug} className="border-r border-b border-line">
						<Link
							href={`/c/${c.slug}`}
							className="group flex h-full flex-col items-center gap-2 px-2 py-4 text-center text-sm text-ink hover:bg-sunken"
						>
							<span className="flex size-14 items-center justify-center rounded-full bg-action-soft text-action-600 group-hover:bg-action-soft-hover">
								<CategoryIcon slug={c.slug} />
							</span>
							<span className="line-clamp-2 break-words">{c.name}</span>
						</Link>
					</li>
				))}
			</ul>
		</nav>
	);
}
