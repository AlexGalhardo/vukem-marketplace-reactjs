import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import { formatBRL, installmentPlan } from "../../lib/money";

// Installments is the "in up to 10x R$ …" line under a price, computed
// from integer cents. Display only; checkout charges the full amount.
export default function Installments({
	cents,
	className = "",
}: {
	cents: number;
	className?: string;
}): JSX.Element | null {
	const { t, locale } = useT();
	const plan = installmentPlan(cents);
	if (!plan) return null;

	return (
		<p className={`text-sm text-ink ${className}`}>
			{t("storefront.price.installments", { count: plan.count, part: formatBRL(plan.partCents, locale) })}
		</p>
	);
}
