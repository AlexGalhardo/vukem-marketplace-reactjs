import type { JSX } from "react";

// Logo is the Vukem Marketplace mark (a parcel folded into a "V") and
// wordmark. Purely our own drawing; screen readers get the plain name.
export default function Logo({ name, compact = false }: { name: string; compact?: boolean }): JSX.Element {
	return (
		<span className="flex items-center gap-2 text-on-header">
			<svg viewBox="0 0 40 40" aria-hidden="true" className="size-9 shrink-0 text-mark">
				<rect
					x="1.5"
					y="1.5"
					width="37"
					height="37"
					rx="10"
					fill="none"
					stroke="currentColor"
					strokeWidth="3"
				/>
				<path
					d="M10 13.5 20 29l10-15.5"
					fill="none"
					stroke="currentColor"
					strokeWidth="3.4"
					strokeLinecap="round"
					strokeLinejoin="round"
				/>
				<path d="M15.2 13.5h9.6" stroke="currentColor" strokeWidth="3.4" strokeLinecap="round" />
			</svg>
			<span className="sr-only">{name}</span>
			<span aria-hidden="true" className={`flex-col leading-none ${compact ? "hidden sm:flex" : "flex"}`}>
				<span className="text-lg font-extrabold tracking-tight">vukem</span>
				<span className="text-[0.7rem] font-medium tracking-wide text-header-muted">marketplace</span>
			</span>
		</span>
	);
}
