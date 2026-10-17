import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import CheckoutSteps from "../../components/commerce/CheckoutSteps";
import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { formatMoney } from "../../lib/money";
import type { CheckoutQuote } from "../../types";

interface Props {
	quote: CheckoutQuote;
	methods: { card: boolean; pix: boolean };
}

type Method = "card" | "pix";

export default function CheckoutPayment({ quote, methods }: Props): JSX.Element {
	const { t, locale } = useT();
	const cardOK = methods.card && quote.acceptsCard;
	const form = useForm<{ method: Method | "" }>({ method: methods.pix ? "pix" : cardOK ? "card" : "" });
	const money = (cents: number) => formatMoney(cents, locale);
	const a = quote.address;

	// Only the method is sent: the server prices the order again.
	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/checkout");
	};

	const option = (method: Method, enabled: boolean, hint: string, off: string) => (
		<label
			className={`flex gap-3 rounded-md border border-line bg-card p-4 ${
				enabled
					? "cursor-pointer has-checked:border-action-500 has-checked:ring-1 has-checked:ring-action-500"
					: "opacity-60"
			}`}
		>
			<input
				type="radio"
				name="method"
				value={method}
				disabled={!enabled}
				checked={form.data.method === method}
				onChange={() => form.setData("method", method)}
				className="mt-1"
			/>
			<span>
				<span className="block font-medium">{t(`checkout.payment.${method}`)}</span>
				<span className="block text-sm text-muted">{enabled ? hint : off}</span>
			</span>
		</label>
	);

	return (
		<>
			<Head title={t("checkout.payment.title")} />
			<CheckoutSteps current="payment" />
			<h1 className="mb-4 text-2xl font-semibold">{t("checkout.payment.title")}</h1>
			<div className="grid gap-6 lg:grid-cols-[1fr_22rem] lg:items-start">
				<div className="flex flex-col gap-4">
					<section className="rounded-md bg-card p-5 shadow-card">
						<div className="flex items-baseline justify-between gap-4">
							<h2 className="text-sm text-muted">{t("checkout.payment.deliver_to")}</h2>
							<Link href="/checkout/address" className="text-sm text-action-600 hover:underline">
								{t("checkout.payment.change")}
							</Link>
						</div>
						<p className="mt-1 font-medium">{a.recipient}</p>
						<p className="text-sm text-muted">
							{a.street}, {a.number}
							{a.complement && ` – ${a.complement}`}, {a.district}, {a.city}/{a.uf}
						</p>
					</section>
					{quote.groups.map((g) => (
						<section key={g.shopId} className="rounded-md bg-card p-5 shadow-card">
							<h2 className="mb-2 text-sm text-muted">
								{t("checkout.payment.from", { shop: g.shopName })}
							</h2>
							<ul className="divide-y divide-line">
								{g.lines.map((l) => (
									<li key={l.id} className="flex justify-between gap-4 py-2 text-sm">
										<span>
											{l.quantity} × {l.title}
										</span>
										<span className="tabular-nums">{money(l.lineTotalCents)}</span>
									</li>
								))}
							</ul>
							<p className="mt-2 text-sm text-success-600">
								{t("checkout.payment.shipping_days", { price: money(g.shippingCents), days: g.days })}
							</p>
						</section>
					))}
				</div>
				<form onSubmit={submit} className="flex flex-col gap-3 lg:sticky lg:top-4">
					<fieldset className="flex flex-col gap-3">
						<legend className="mb-1 font-semibold">{t("checkout.payment.method")}</legend>
						{option("pix", methods.pix, t("checkout.payment.pix_hint"), t("checkout.payment.pix_off"))}
						{option(
							"card",
							cardOK,
							t("checkout.payment.card_hint"),
							methods.card ? t("checkout.payment.card_seller") : t("checkout.payment.card_off"),
						)}
					</fieldset>
					<dl className="grid grid-cols-[1fr_auto] gap-y-1 rounded-md bg-card p-5 shadow-card text-sm">
						<dt className="text-muted">{t("checkout.payment.products")}</dt>
						<dd className="text-right tabular-nums">{money(quote.subtotalCents)}</dd>
						<dt className="text-muted">{t("checkout.payment.shipping")}</dt>
						<dd className="text-right tabular-nums">{money(quote.shippingCents)}</dd>
						<dt className="pt-2 text-base font-semibold">{t("checkout.payment.total")}</dt>
						<dd className="pt-2 text-right text-base font-semibold tabular-nums">
							{money(quote.totalCents)}
						</dd>
					</dl>
					{form.errors.method && (
						<p role="alert" className="text-sm text-danger-600">
							{serverMessage(locale, form.errors.method)}
						</p>
					)}
					<button
						type="submit"
						disabled={form.processing || form.data.method === ""}
						className="rounded-md bg-action-500 px-6 py-3 font-semibold text-on-action hover:bg-action-600 disabled:opacity-50"
					>
						{t("checkout.payment.place", { total: money(quote.totalCents) })}
					</button>
				</form>
			</div>
		</>
	);
}
