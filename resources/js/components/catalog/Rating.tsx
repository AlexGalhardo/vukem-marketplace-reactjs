import { type JSX, useId } from "react";

import { useT } from "../../lib/i18n";

// Rating draws five stars filled to the average (stored ×100) and the
// review count. Screen readers get one sentence instead of the glyphs.
export default function Rating({
	avgX100,
	count,
	size = "sm",
}: {
	avgX100: number;
	count: number;
	size?: "sm" | "md";
}): JSX.Element | null {
	const { t, locale } = useT();
	const id = `star${useId().replace(/[^a-zA-Z0-9]/g, "")}`;
	if (count <= 0) return null;

	const avg = Math.max(0, Math.min(500, avgX100)) / 100;
	const label = t("catalog.product.rating", {
		avg: avg.toLocaleString(locale, { maximumFractionDigits: 1 }),
		count,
	});
	const star = size === "md" ? "size-4" : "size-3";

	return (
		<span className={`flex items-center gap-1 ${size === "md" ? "text-sm" : "text-xs"} text-muted`}>
			<span className="sr-only">{label}</span>
			{size === "md" && (
				<span aria-hidden="true" className="text-ink tabular-nums">
					{avg.toLocaleString(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}
				</span>
			)}
			<span aria-hidden="true" className="flex text-star">
				{[0, 1, 2, 3, 4].map((i) => {
					const fill = Math.max(0, Math.min(1, avg - i));
					return (
						<svg key={i} viewBox="0 0 20 20" aria-hidden="true" className={star}>
							<defs>
								<linearGradient id={`${id}-${i}`}>
									<stop offset={fill} stopColor="currentColor" />
									<stop offset={fill} stopColor="currentColor" stopOpacity="0.25" />
								</linearGradient>
							</defs>
							<path
								fill={`url(#${id}-${i})`}
								d="m10 1.8 2.5 5.2 5.7.8-4.1 4 1 5.7L10 14.8l-5.1 2.7 1-5.7-4.1-4 5.7-.8z"
							/>
						</svg>
					);
				})}
			</span>
			<span aria-hidden="true" className="tabular-nums">
				({count})
			</span>
		</span>
	);
}
