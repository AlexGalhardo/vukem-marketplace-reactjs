import { Head, Link, router } from "@inertiajs/react";
import { type JSX, useEffect, useId, useState } from "react";

import CheckoutSteps from "../../components/commerce/CheckoutSteps";
import { useT } from "../../lib/i18n";
import { formatDateTime, formatMoney } from "../../lib/money";
import type { OrderStatus, OrderSummary } from "../../types";

interface Props {
	order: OrderSummary;
	brCode: string;
	// A data: URI (PNG from the provider or an SVG drawn by the server), shown only as an image.
	qr: string;
	canSimulate: boolean;
}

const pollMs = 4000;

// shortcut: polls the order status; the realtime phase (O.2) moves this to SSE.
function useOrderStatus(publicId: string, initial: OrderStatus): OrderStatus {
	const [status, setStatus] = useState(initial);

	useEffect(() => {
		if (status !== "pending") return;
		const timer = window.setInterval(async () => {
			try {
				const res = await fetch(`/checkout/pix/${publicId}/status`, {
					headers: { Accept: "application/json" },
				});
				if (!res.ok) return;
				const body = (await res.json()) as { status: OrderStatus };
				setStatus(body.status);
			} catch {
				// Offline for a moment: the next tick tries again.
			}
		}, pollMs);
		return () => window.clearInterval(timer);
	}, [publicId, status]);

	return status;
}

export default function CheckoutPix({ order, brCode, qr, canSimulate }: Props): JSX.Element {
	const { t, locale } = useT();
	const codeId = useId();
	const status = useOrderStatus(order.publicId, order.status);
	const [copied, setCopied] = useState(false);

	const copy = async () => {
		try {
			await navigator.clipboard.writeText(brCode);
			setCopied(true);
			window.setTimeout(() => setCopied(false), 2000);
		} catch {
			document.getElementById(codeId)?.focus();
		}
	};

	return (
		<>
			<Head title={t("checkout.pix.title")} />
			<CheckoutSteps current="pay" />
			<div className="mx-auto max-w-xl">
				<h1 className="text-2xl font-semibold">{t("checkout.pix.title")}</h1>
				<p className="mt-1 text-3xl font-bold tabular-nums">{formatMoney(order.totalCents, locale)}</p>

				{status === "pending" && (
					<div className="mt-6 flex flex-col items-center gap-5 rounded-md bg-card p-6 shadow-card">
						<p className="text-center text-sm text-muted">{t("checkout.pix.instructions")}</p>
						{qr ? (
							<img
								src={qr}
								alt={t("checkout.pix.qr_alt")}
								width={256}
								height={256}
								className="rounded-md bg-white p-3"
							/>
						) : (
							<p className="text-sm text-muted">{t("checkout.pix.no_qr")}</p>
						)}
						<div className="flex w-full flex-col gap-2">
							<label htmlFor={codeId} className="text-sm font-medium">
								{t("checkout.pix.code")}
							</label>
							<textarea
								id={codeId}
								readOnly
								rows={3}
								value={brCode}
								onFocus={(e) => e.currentTarget.select()}
								className="w-full resize-none rounded-md border border-line bg-surface p-3 font-mono text-xs break-all text-ink"
							/>
							<button
								type="button"
								onClick={copy}
								className="rounded-md bg-action-500 px-4 py-3 font-semibold text-on-action hover:bg-action-600"
							>
								{copied ? t("checkout.pix.copied") : t("checkout.pix.copy")}
							</button>
						</div>
						{order.pixExpiresAt && (
							<p className="text-sm text-muted">
								{t("checkout.pix.expires", { time: formatDateTime(order.pixExpiresAt, locale) })}
							</p>
						)}
						<p role="status" aria-live="polite" className="text-sm">
							{t("checkout.pix.waiting")}
						</p>
						{canSimulate && (
							<button
								type="button"
								onClick={() => router.post(`/checkout/pix/${order.publicId}/simulate`)}
								className="text-sm text-action-600 underline"
							>
								{t("checkout.pix.simulate")}
							</button>
						)}
					</div>
				)}

				{status !== "pending" && (
					<div role="status" className="mt-6 rounded-md bg-card p-6 shadow-card">
						<p
							className={
								status === "paid" ? "font-semibold text-success-600" : "font-semibold text-danger-600"
							}
						>
							{status === "paid" ? t("checkout.pix.paid") : t("checkout.pix.expired")}
						</p>
						<Link
							href={status === "paid" ? "/" : "/cart"}
							className="mt-3 inline-block text-action-600 hover:underline"
						>
							{status === "paid" ? t("checkout.pix.back") : t("cart.title")}
						</Link>
					</div>
				)}
			</div>
		</>
	);
}
