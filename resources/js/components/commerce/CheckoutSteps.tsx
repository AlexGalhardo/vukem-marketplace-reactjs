import type { JSX } from "react";

import { useT } from "../../lib/i18n";

const steps = ["address", "payment", "pay"] as const;

// CheckoutSteps shows where the buyer is in the checkout (a real sequence).
export default function CheckoutSteps({ current }: { current: (typeof steps)[number] }): JSX.Element {
	const { t } = useT();
	const at = steps.indexOf(current);

	return (
		<nav aria-label={t("checkout.steps.label")} className="mb-6">
			<ol className="flex items-center gap-2 text-sm">
				{steps.map((step, i) => (
					<li key={step} className="flex items-center gap-2" aria-current={i === at ? "step" : undefined}>
						<span
							className={`flex size-6 items-center justify-center rounded-full text-xs font-semibold ${
								i <= at ? "bg-action-500 text-on-action" : "border border-line text-muted"
							}`}
						>
							{i + 1}
						</span>
						<span className={i === at ? "font-semibold text-ink" : "text-muted"}>
							{t(`checkout.steps.${step}`)}
						</span>
						{i < steps.length - 1 && <span aria-hidden="true" className="mx-1 h-px w-8 bg-line" />}
					</li>
				))}
			</ol>
		</nav>
	);
}
