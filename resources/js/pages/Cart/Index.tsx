import { Head, Link, router, usePage } from "@inertiajs/react";
import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { formatMoney } from "../../lib/money";
import type { CartLine, CartView, SharedProps } from "../../types";

interface Props {
	cart: CartView;
}

function groupByShop(lines: CartLine[]): { shopName: string; shopSlug: string; lines: CartLine[] }[] {
	const groups = new Map<number, { shopName: string; shopSlug: string; lines: CartLine[] }>();
	for (const line of lines) {
		const g = groups.get(line.shopId) ?? { shopName: line.shopName, shopSlug: line.shopSlug, lines: [] };
		g.lines.push(line);
		groups.set(line.shopId, g);
	}
	return [...groups.values()];
}

function LineRow({ line }: { line: CartLine }): JSX.Element {
	const { t, locale } = useT();
	const max = Math.max(1, Math.min(99, line.stock || line.quantity));

	const setQuantity = (quantity: number) =>
		router.patch(`/cart/items/${line.id}`, { quantity }, { preserveScroll: true });

	return (
		<li className="flex flex-wrap items-center gap-4 py-4">
			<div className="min-w-0 flex-1">
				<Link href={`/p/${line.slug}`} className="font-medium text-ink hover:text-action-600">
					{line.title}
				</Link>
				<p className="text-sm text-muted">{t("cart.each", { price: formatMoney(line.priceCents, locale) })}</p>
				{line.issue !== "" && (
					<p className="mt-1 text-sm font-medium text-danger-600">
						{t(`cart.issue.${line.issue}`, { stock: line.stock })}
					</p>
				)}
			</div>
			<label className="flex items-center gap-2 text-sm text-muted">
				{t("cart.quantity")}
				<select
					value={line.quantity}
					onChange={(e) => setQuantity(Number(e.target.value))}
					className="h-9 rounded-md border border-line bg-card px-2 text-ink"
				>
					{Array.from({ length: Math.max(max, line.quantity) }, (_, i) => i + 1).map((n) => (
						<option key={n} value={n}>
							{n}
						</option>
					))}
				</select>
			</label>
			<p className="w-28 text-right font-semibold tabular-nums">{formatMoney(line.lineTotalCents, locale)}</p>
			<button
				type="button"
				onClick={() => router.delete(`/cart/items/${line.id}`, { preserveScroll: true })}
				className="text-sm text-action-600 hover:underline"
			>
				{t("cart.remove")}
			</button>
		</li>
	);
}

export default function CartIndex({ cart }: Props): JSX.Element {
	const { t, locale } = useT();
	const errors = usePage<SharedProps>().props.errors ?? {};
	const error = serverMessage(locale, errors.quantity ?? errors.product);

	return (
		<>
			<Head title={t("cart.title")} />
			<h1 className="mb-4 text-2xl font-semibold">{t("cart.title")}</h1>
			{error && (
				<p role="alert" className="mb-4 text-sm text-danger-600">
					{error}
				</p>
			)}
			{cart.lines.length === 0 ? (
				<div className="rounded-md bg-card p-8 text-center">
					<p className="mb-4 text-muted">{t("cart.empty")}</p>
					<Link href="/" className="font-medium text-action-600 hover:underline">
						{t("cart.empty_cta")}
					</Link>
				</div>
			) : (
				<div className="grid gap-6 lg:grid-cols-[1fr_20rem] lg:items-start">
					<div className="flex flex-col gap-4">
						{cart.hasIssues && (
							<p role="status" className="rounded-md bg-danger-600/10 px-4 py-3 text-sm text-ink">
								{t("cart.has_issues")}
							</p>
						)}
						{groupByShop(cart.lines).map((g) => (
							<section key={g.shopSlug} className="rounded-md bg-card px-4">
								<h2 className="border-b border-line py-3 text-sm text-muted">
									{t("cart.sold_by", { shop: g.shopName })}
								</h2>
								<ul className="divide-y divide-line">
									{g.lines.map((line) => (
										<LineRow key={line.id} line={line} />
									))}
								</ul>
							</section>
						))}
					</div>
					<aside className="rounded-md bg-card p-4 lg:sticky lg:top-4">
						<dl className="flex justify-between text-sm">
							<dt className="text-muted">{t("cart.subtotal", { count: cart.itemCount })}</dt>
							<dd className="font-semibold tabular-nums">{formatMoney(cart.subtotalCents, locale)}</dd>
						</dl>
						<p className="mt-2 text-xs text-muted">{t("cart.shipping_note")}</p>
						{cart.hasIssues || cart.itemCount === 0 ? (
							<span
								aria-disabled="true"
								className="mt-4 block rounded-md bg-action-500 px-4 py-3 text-center font-semibold text-white opacity-50"
							>
								{t("cart.checkout")}
							</span>
						) : (
							<Link
								href="/checkout"
								className="mt-4 block rounded-md bg-action-500 px-4 py-3 text-center font-semibold text-white hover:bg-action-600"
							>
								{t("cart.checkout")}
							</Link>
						)}
					</aside>
				</div>
			)}
		</>
	);
}
