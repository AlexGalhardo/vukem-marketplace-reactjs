import { Head, useForm, usePage } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { formatDateTime, formatMoney } from "../../lib/money";
import type { PayoutRow, SharedProps } from "../../types";

interface Props {
	pending: PayoutRow[];
	paid: PayoutRow[];
}

function PendingRow({ payout }: { payout: PayoutRow }): JSX.Element {
	const { t, locale } = useT();
	const pay = useForm({ endToEndId: "" });
	const reject = useForm({ reason: "" });

	const submitPay = (e: FormEvent) => {
		e.preventDefault();
		pay.post(`/admin/payouts/${payout.id}/paid`, { preserveScroll: true });
	};

	const submitReject = (e: FormEvent) => {
		e.preventDefault();
		reject.post(`/admin/payouts/${payout.id}/reject`, { preserveScroll: true });
	};

	return (
		<li className="grid gap-3 px-4 py-4 lg:grid-cols-[1fr_auto_auto] lg:items-start">
			<div>
				<p className="font-medium">{payout.shopName}</p>
				<p className="text-sm text-muted">
					{t("checkout.admin_payouts.requested_at")} {formatDateTime(payout.createdAt, locale)}
				</p>
				<p className="mt-1 text-lg font-semibold tabular-nums">{formatMoney(payout.amountCents, locale)}</p>
			</div>
			<form onSubmit={submitPay} className="flex flex-col gap-1">
				<label htmlFor={`e2e-${payout.id}`} className="text-sm">
					{t("checkout.admin_payouts.e2e")}
				</label>
				<div className="flex gap-2">
					<input
						id={`e2e-${payout.id}`}
						required
						maxLength={32}
						value={pay.data.endToEndId}
						onChange={(e) => pay.setData("endToEndId", e.target.value)}
						className="h-10 w-72 rounded-md border border-line bg-card px-3 font-mono text-xs text-ink"
					/>
					<button
						type="submit"
						disabled={pay.processing}
						className="h-10 rounded-md bg-action-500 px-4 text-sm font-semibold text-on-action hover:bg-action-600 disabled:opacity-50"
					>
						{t("checkout.admin_payouts.mark_paid")}
					</button>
				</div>
				{pay.errors.endToEndId && (
					<p role="alert" className="text-sm text-danger-600">
						{serverMessage(locale, pay.errors.endToEndId)}
					</p>
				)}
			</form>
			<form onSubmit={submitReject} className="flex flex-col gap-1">
				<label htmlFor={`reason-${payout.id}`} className="text-sm">
					{t("checkout.admin_payouts.reason")}
				</label>
				<div className="flex gap-2">
					<input
						id={`reason-${payout.id}`}
						required
						minLength={3}
						maxLength={500}
						value={reject.data.reason}
						onChange={(e) => reject.setData("reason", e.target.value)}
						className="h-10 w-48 rounded-md border border-line bg-card px-3 text-sm text-ink"
					/>
					<button
						type="submit"
						disabled={reject.processing}
						className="h-10 rounded-md border border-danger-600 px-4 text-sm text-danger-600 hover:bg-danger-600/10 disabled:opacity-50"
					>
						{t("checkout.admin_payouts.reject")}
					</button>
				</div>
			</form>
		</li>
	);
}

export default function AdminPayouts({ pending, paid }: Props): JSX.Element {
	const { t, locale } = useT();
	const errors = usePage<SharedProps>().props.errors ?? {};

	return (
		<>
			<Head title={t("checkout.admin_payouts.title")} />
			<h1 className="text-2xl font-semibold">{t("checkout.admin_payouts.title")}</h1>
			<p className="mt-1 mb-6 max-w-2xl text-muted">{t("checkout.admin_payouts.intro")}</p>
			{errors.payout && (
				<p role="alert" className="mb-4 text-sm text-danger-600">
					{serverMessage(locale, errors.payout)}
				</p>
			)}
			{pending.length === 0 ? (
				<p className="text-muted">{t("checkout.admin_payouts.none")}</p>
			) : (
				<ul className="divide-y divide-line rounded-md bg-card">
					{pending.map((p) => (
						<PendingRow key={p.id} payout={p} />
					))}
				</ul>
			)}
			<h2 className="mt-8 mb-3 text-lg font-semibold">{t("checkout.admin_payouts.paid_title")}</h2>
			<table className="w-full text-left text-sm">
				<thead className="text-muted">
					<tr>
						<th className="py-2 font-normal">{t("checkout.admin_payouts.shop")}</th>
						<th className="py-2 font-normal">{t("checkout.admin_payouts.amount")}</th>
						<th className="py-2 font-normal">{t("checkout.admin_payouts.e2e")}</th>
						<th className="py-2 font-normal">{t("checkout.admin_payouts.paid_at")}</th>
					</tr>
				</thead>
				<tbody className="divide-y divide-line">
					{paid.map((p) => (
						<tr key={p.id}>
							<td className="py-2">{p.shopName}</td>
							<td className="py-2 tabular-nums">{formatMoney(p.amountCents, locale)}</td>
							<td className="py-2 font-mono text-xs">{p.endToEndId}</td>
							<td className="py-2">{p.paidAt ? formatDateTime(p.paidAt, locale) : ""}</td>
						</tr>
					))}
				</tbody>
			</table>
		</>
	);
}
