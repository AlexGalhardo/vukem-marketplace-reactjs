import { Link } from "@inertiajs/react";
import type { JSX } from "react";

import type { CategoryRef } from "../../types";

// CategoryRail is a horizontally scrolling row of category links.
export default function CategoryRail({
	categories,
	active,
	label,
}: {
	categories: CategoryRef[];
	active?: string;
	label: string;
}): JSX.Element {
	return (
		<nav aria-label={label} className="-mx-4 overflow-x-auto px-4 [scrollbar-width:thin]">
			<ul className="flex gap-2 pb-1">
				{categories.map((c) => (
					<li key={c.slug} className="shrink-0">
						<Link
							href={`/c/${c.slug}`}
							aria-current={c.slug === active ? "page" : undefined}
							className="block rounded-full border border-line bg-card px-4 py-2 text-sm text-ink hover:border-action-500 hover:text-action-600 aria-[current=page]:border-action-600 aria-[current=page]:bg-action-600 aria-[current=page]:text-on-action"
						>
							{c.name}
						</Link>
					</li>
				))}
			</ul>
		</nav>
	);
}
