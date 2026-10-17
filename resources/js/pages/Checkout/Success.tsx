import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import { formatMoney } from "../../lib/money";
import type { OrderSummary } from "../../types";

// CheckoutSuccess is where Stripe sends the buyer back. It shows the order
// as the server knows it: only a verified webhook marks it paid.
export default function CheckoutSuccess({ order }: { order: OrderSummary }): JSX.Element {
	const { t, locale } = useT();
	const message =
		order.status === "paid"
			? t("checkout.success.paid")
			: order.status === "pending"
				? t("checkout.success.pending")
				: t("checkout.success.closed");

	return (
		<>
			<Head title={t("checkout.success.title")} />
			<div className="mx-auto max-w-xl rounded-md bg-card p-6">
				<h1 className="text-2xl font-semibold">{t("checkout.success.title")}</h1>
				<p role="status" className="mt-2">
					{message}
				</p>
				<p className="mt-4 text-sm text-muted">{t("checkout.success.order", { id: order.publicId })}</p>
				<p className="text-sm">{t(`checkout.status.${order.status}`)}</p>
				<p className="mt-2 font-semibold tabular-nums">
					{t("checkout.success.total")}: {formatMoney(order.totalCents, locale)}
				</p>
				<Link href="/" className="mt-6 inline-block text-action-600 hover:underline">
					{t("checkout.pix.back")}
				</Link>
			</div>
		</>
	);
}
