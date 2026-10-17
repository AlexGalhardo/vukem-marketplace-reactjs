import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import { formatBRL, priceParts } from "../../lib/money";

const sizes = {
	sm: { whole: "text-2xl", fraction: "text-xs mt-0.5" },
	md: { whole: "text-[1.75rem]", fraction: "text-sm mt-0.5" },
	lg: { whole: "text-4xl", fraction: "text-lg mt-1" },
} as const;

// Price shows BRL the way marketplace shoppers scan it: the whole amount
// large and light, the cents raised and small. Screen readers get the
// plain formatted amount.
export default function Price({ cents, size = "md" }: { cents: number; size?: keyof typeof sizes }): JSX.Element {
	const { locale } = useT();
	const p = priceParts(cents, locale);
	const s = sizes[size];

	return (
		<p className="relative flex items-start font-normal tracking-tight text-ink tabular-nums">
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
