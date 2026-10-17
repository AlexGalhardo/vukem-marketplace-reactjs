import { Head, router, useForm, usePage } from "@inertiajs/react";
import { type FormEvent, type JSX, useState } from "react";

import Field from "../../components/Field";
import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { formatDateTime, formatMoney } from "../../lib/money";
import type { BalanceSummary, PayoutRow, SharedProps } from "../../types";

interface Props {
	shop: { id: number; name: string; slug: string; stripeReady: boolean };
	summary: BalanceSummary;
	payouts: PayoutRow[];
	stripe: { enabled: boolean; connected: boolean; ready: boolean };
	minPayoutCents: number;
}

// toCents parses "123,45" or "123.45" into integer cents without floats.
function toCents(input: string): number {
	const m = input.trim().match(/^(\d{1,9})(?:[.,](\d{1,2}))?$/);
	if (!m) return 0;
	return Number(m[1]) * 100 + Number((m[2] ?? "0").padEnd(2, "0"));
}

export default function SellerPayouts({ shop, summary, payouts, stripe, minPayoutCents }: Props): JSX.Element {
	const { t, locale } = useT();
	const errors = usePage<SharedProps>().props.errors ?? {};
	const [amount, setAmount] = useState("");
	const form = useForm({ amountCents: 0 });
	const money = (cents: number) => formatMoney(cents, locale);

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.transform(() => ({ amountCents: toCents(amount) }));
		form.post("/seller/payouts", { preserveScroll: true, onSuccess: () => setAmount("") });
	};

	return (
		<>
			<Head title={t("checkout.payouts.title")} />
			<h1 className="mb-1 text-2xl font-semibold">{t("checkout.payouts.title")}</h1>
			<p className="mb-6 text-muted">{shop.name}</p>

			<div className="grid gap-6 lg:grid-cols-2">
				<section className="rounded-md bg-card p-5 shadow-card">
					<h2 className="text-sm text-muted">{t("checkout.payouts.balance")}</h2>
					<p className="mt-1 text-3xl font-bold tabular-nums">{money(summary.balanceCents)}</p>
					<dl className="mt-3 grid grid-cols-[1fr_auto] gap-y-1 text-sm">
						<dt className="text-muted">{t("checkout.payouts.requested")}</dt>
						<dd className="text-right tabular-nums">{money(summary.requestedCents)}</dd>
						<dt className="text-muted">{t("checkout.payouts.available")}</dt>
						<dd className="text-right font-semibold tabular-nums">{money(summary.availableCents)}</dd>
					</dl>
					<form onSubmit={submit} className="mt-5 flex flex-col gap-3">
						<Field
							label={t("checkout.payouts.amount")}
							inputMode="decimal"
							required
							value={amount}
							onChange={(e) => setAmount(e.target.value)}
							error={serverMessage(locale, form.errors.amountCents)}
							aria-describedby="payout-min"
						/>
						<p id="payout-min" className="text-xs text-muted">
							{t("checkout.payouts.min", { amount: money(minPayoutCents) })}
						</p>
						<button
							type="submit"
							disabled={form.processing || summary.availableCents < minPayoutCents}
							className="self-start rounded-md bg-action-500 px-5 py-2 font-semibold text-on-action hover:bg-action-600 disabled:opacity-50"
						>
							{t("checkout.payouts.request")}
						</button>
					</form>
				</section>

				<section className="rounded-md bg-card p-5 shadow-card">
					<h2 className="font-semibold">{t("checkout.payouts.card_title")}</h2>
					<p className="mt-2 text-sm text-muted">
						{!stripe.enabled
							? t("checkout.payouts.card_off")
							: stripe.ready
								? t("checkout.payouts.card_ready")
								: t("checkout.payouts.card_pending")}
					</p>
					{stripe.enabled && !stripe.ready && (
						<button
							type="button"
							onClick={() => router.post("/seller/payouts/stripe")}
							className="mt-4 rounded-md border border-action-500 px-5 py-2 font-medium text-action-600 hover:bg-action-500/10"
						>
							{stripe.connected
								? t("checkout.payouts.card_continue")
								: t("checkout.payouts.card_connect")}
						</button>
					)}
					{errors.stripe && (
						<p role="alert" className="mt-2 text-sm text-danger-600">
							{serverMessage(locale, errors.stripe)}
						</p>
					)}
				</section>
			</div>

			<section className="mt-8">
				<h2 className="mb-3 text-lg font-semibold">{t("checkout.payouts.history")}</h2>
				{payouts.length === 0 ? (
					<p className="text-muted">{t("checkout.payouts.none")}</p>
				) : (
					<ul className="divide-y divide-line rounded-md bg-card shadow-card">
						{payouts.map((p) => (
							<li
								key={p.id}
								className="flex flex-wrap items-center justify-between gap-3 px-4 py-3 text-sm"
							>
								<span className="tabular-nums">{formatDateTime(p.createdAt, locale)}</span>
								<span className="font-semibold tabular-nums">{money(p.amountCents)}</span>
								<span
									className={
										p.status === "rejected"
											? "text-danger-600"
											: p.status === "paid"
												? "text-success-600"
												: "text-muted"
									}
								>
									{t(`checkout.payouts.status.${p.status}`)}
									{p.rejectedReason && `: ${p.rejectedReason}`}
								</span>
							</li>
						))}
					</ul>
				)}
			</section>
		</>
	);
}
