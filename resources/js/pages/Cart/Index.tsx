import { Head, Link, router, usePage } from "@inertiajs/react";
import type { JSX } from "react";
import ProductImage from "../../components/catalog/ProductImage";
import { CartIcon, StoreIcon } from "../../components/icons";
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
	const step =
		"flex size-8 items-center justify-center text-lg text-action-600 disabled:text-muted disabled:opacity-50";

	return (
		<li className="grid grid-cols-[4rem_1fr] gap-x-4 gap-y-3 py-5 sm:grid-cols-[4rem_1fr_auto_7.5rem] sm:items-center">
			<div className="size-16 overflow-hidden rounded-md border border-line">
				<ProductImage src="" alt="" />
			</div>
			<div className="min-w-0">
				<Link href={`/p/${line.slug}`} className="line-clamp-2 break-words text-ink hover:text-action-600">
					{line.title}
				</Link>
				<p className="text-sm text-muted">{t("cart.each", { price: formatMoney(line.priceCents, locale) })}</p>
				{line.issue !== "" && (
					<p className="mt-1 text-sm font-medium text-danger-600">
						{t(`cart.issue.${line.issue}`, { stock: line.stock })}
					</p>
				)}
				<button
					type="button"
					onClick={() => router.delete(`/cart/items/${line.id}`, { preserveScroll: true })}
					className="mt-1 text-sm font-medium text-action-600 hover:underline"
				>
					{t("cart.remove")}
				</button>
			</div>
			<div className="col-start-2 flex flex-col items-start gap-1 sm:col-start-auto sm:items-center">
				<fieldset className="flex items-center rounded-md border border-line">
					<legend className="sr-only">{t("cart.quantity")}</legend>
					<button
						type="button"
						disabled={line.quantity <= 1}
						onClick={() => setQuantity(line.quantity - 1)}
						aria-label={t("storefront.cart.decrease")}
						className={step}
					>
						−
					</button>
					<label className="sr-only" htmlFor={`qty-${line.id}`}>
						{t("cart.quantity")}
					</label>
					<select
						id={`qty-${line.id}`}
						value={line.quantity}
						onChange={(e) => setQuantity(Number(e.target.value))}
						className="h-8 appearance-none bg-transparent px-1 text-center text-sm text-ink tabular-nums focus:outline-none"
					>
						{Array.from({ length: Math.max(max, line.quantity) }, (_, i) => i + 1).map((n) => (
							<option key={n} value={n}>
								{n}
							</option>
						))}
					</select>
					<button
						type="button"
						disabled={line.quantity >= max}
						onClick={() => setQuantity(line.quantity + 1)}
						aria-label={t("storefront.cart.increase")}
						className={step}
					>
						+
					</button>
				</fieldset>
				{line.stock > 0 && (
					<p className="text-xs text-muted">{t("storefront.cart.available", { count: line.stock })}</p>
				)}
			</div>
			<p className="col-start-2 text-lg font-normal text-ink tabular-nums sm:col-start-auto sm:text-right">
				{formatMoney(line.lineTotalCents, locale)}
			</p>
		</li>
	);
}

export default function CartIndex({ cart }: Props): JSX.Element {
	const { t, locale } = useT();
	const errors = usePage<SharedProps>().props.errors ?? {};
	const error = serverMessage(locale, errors.quantity ?? errors.product);
	const cta = "mt-5 flex h-12 items-center justify-center rounded-md bg-action-500 px-4 font-semibold text-on-action";

	return (
		<>
			<Head title={t("cart.title")} />
			<h1 className="mb-4 text-2xl font-semibold text-ink">{t("cart.title")}</h1>
			{error && (
				<p role="alert" className="mb-4 text-sm text-danger-600">
					{error}
				</p>
			)}
			{cart.lines.length === 0 ? (
				<div className="flex flex-col items-center gap-3 rounded-md bg-card px-6 py-12 text-center shadow-card">
					<span className="flex size-16 items-center justify-center rounded-full bg-action-soft text-action-600">
						<CartIcon className="size-8" />
					</span>
					<p className="text-lg font-semibold text-ink">{t("cart.empty")}</p>
					<Link
						href="/"
						className="inline-flex h-10 items-center rounded-md bg-action-500 px-5 text-sm font-semibold text-on-action hover:bg-action-600"
					>
						{t("cart.empty_cta")}
					</Link>
				</div>
			) : (
				<div className="grid gap-4 lg:grid-cols-[1fr_20rem] lg:items-start">
					<div className="flex flex-col gap-4">
						{cart.hasIssues && (
							<p
								role="status"
								className="rounded-md bg-card px-4 py-3 text-sm text-danger-600 shadow-card"
							>
								{t("cart.has_issues")}
							</p>
						)}
						{groupByShop(cart.lines).map((g) => (
							<section key={g.shopSlug} className="rounded-md bg-card px-4 shadow-card sm:px-6">
								<h2 className="flex items-center gap-2 border-b border-line py-4 text-sm font-semibold text-ink">
									<StoreIcon className="size-5 text-muted" />
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
					<aside
						aria-labelledby="summary-title"
						className="rounded-md bg-card p-5 shadow-card lg:sticky lg:top-4"
					>
						<h2 id="summary-title" className="mb-4 border-b border-line pb-4 font-semibold text-ink">
							{t("storefront.cart.summary")}
						</h2>
						<dl className="grid grid-cols-[1fr_auto] gap-y-2 text-sm">
							<dt className="text-ink">{t("cart.subtotal", { count: cart.itemCount })}</dt>
							<dd className="text-right tabular-nums">{formatMoney(cart.subtotalCents, locale)}</dd>
							<dt className="text-ink">{t("checkout.payment.shipping")}</dt>
							<dd className="text-right text-muted">{t("storefront.cart.shipping_later")}</dd>
							<dt className="pt-3 text-base font-semibold">{t("checkout.payment.total")}</dt>
							<dd className="pt-3 text-right text-base font-semibold tabular-nums">
								{formatMoney(cart.subtotalCents, locale)}
							</dd>
						</dl>
						<p className="mt-3 text-xs text-muted">{t("cart.shipping_note")}</p>
						{cart.hasIssues || cart.itemCount === 0 ? (
							<span aria-disabled="true" className={`${cta} opacity-50`}>
								{t("cart.checkout")}
							</span>
						) : (
							<Link href="/checkout" className={`${cta} hover:bg-action-600`}>
								{t("cart.checkout")}
							</Link>
						)}
					</aside>
				</div>
			)}
		</>
	);
}
