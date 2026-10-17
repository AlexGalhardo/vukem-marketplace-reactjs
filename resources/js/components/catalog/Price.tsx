import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import { formatBRL, priceParts } from "../../lib/money";

const sizes = {
	sm: { whole: "text-xl", fraction: "text-[0.6rem]" },
	md: { whole: "text-2xl", fraction: "text-xs" },
	lg: { whole: "text-4xl", fraction: "text-base" },
} as const;

// Price shows BRL the way marketplace shoppers scan it: the whole amount
// large and light, the cents raised and small. Screen readers get the
// plain formatted amount.
export default function Price({ cents, size = "md" }: { cents: number; size?: keyof typeof sizes }): JSX.Element {
	const { locale } = useT();
	const p = priceParts(cents, locale);
	const s = sizes[size];

	return (
		<p className="flex items-start font-light tracking-tight text-ink tabular-nums">
			<span className="sr-only">{formatBRL(cents, locale)}</span>
			<span aria-hidden="true" className={`${s.whole} mr-1 leading-none`}>
				{p.symbol}
			</span>
			<span aria-hidden="true" className={`${s.whole} leading-none`}>
				{p.whole}
			</span>
			<span aria-hidden="true" className={`${s.fraction} ml-0.5 leading-none`}>
				{p.fraction}
			</span>
		</p>
	);
}
