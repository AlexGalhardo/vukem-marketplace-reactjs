import type { JSX } from "react";

const path = "m10 1.8 2.5 5.2 5.7.8-4.1 4 1 5.7L10 14.8l-5.1 2.7 1-5.7-4.1-4 5.7-.8z";

// Stars draws a whole-star rating (1–5). Decorative: the caller gives the
// accessible text.
export default function Stars({ rating, className = "size-4" }: { rating: number; className?: string }): JSX.Element {
	return (
		<span aria-hidden="true" className="flex text-star">
			{[1, 2, 3, 4, 5].map((n) => (
				<svg key={n} viewBox="0 0 20 20" aria-hidden="true" className={className}>
					<path d={path} fill="currentColor" fillOpacity={n <= rating ? 1 : 0.25} />
				</svg>
			))}
		</span>
	);
}
