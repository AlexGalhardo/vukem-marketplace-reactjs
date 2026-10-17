import { Link, router, usePage } from "@inertiajs/react";
import { type FormEvent, type JSX, useId, useState } from "react";

import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import type { SharedProps } from "../../types";

interface Props {
	slug: string;
	// Units in stock; the server checks again on every add.
	stock: number;
}

// AddToCartButton is the product page's buy box: quantity and the add
// button. It only sends the slug and quantity; the server prices the cart.
export default function AddToCartButton({ slug, stock }: Props): JSX.Element {
	const page = usePage<SharedProps>();
	const { t, locale } = useT();
	const id = useId();
	const [quantity, setQuantity] = useState(1);
	const [busy, setBusy] = useState(false);
	const max = Math.max(1, Math.min(99, stock));
	const error =
		serverMessage(locale, page.props.errors?.quantity) ?? serverMessage(locale, page.props.errors?.product);

	if (!page.props.auth?.user) {
		return (
			<Link
				href="/login"
				className="inline-flex h-12 items-center justify-center rounded-md bg-action-500 px-6 font-semibold text-on-action hover:bg-action-600"
			>
				{t("cart.login_to_buy")}
			</Link>
		);
	}

	const add = (then?: () => void): void => {
		router.post(
			"/cart/items",
			{ slug, quantity },
			{
				preserveScroll: true,
				onStart: () => setBusy(true),
				onFinish: () => setBusy(false),
				onSuccess: then,
			},
		);
	};

	const submit = (e: FormEvent) => {
		e.preventDefault();
		add();
	};

	// Buy now is "add to cart, then go to checkout" on the same endpoints;
	// the server still prices everything.
	const buyNow = (): void => add(() => router.visit("/checkout"));

	return (
		<form onSubmit={submit} className="flex flex-col gap-3">
			<div className="flex items-center gap-2">
				<label htmlFor={id} className="text-sm text-ink">
					{t("cart.quantity")}
				</label>
				<input
					id={id}
					type="number"
					inputMode="numeric"
					min={1}
					max={max}
					value={quantity}
					onChange={(e) => setQuantity(Math.min(max, Math.max(1, Number(e.target.value) || 1)))}
					className="h-9 w-20 rounded-md border border-line bg-card px-3 text-ink focus:border-action-500 focus:outline-none"
				/>
			</div>
			<button
				type="button"
				onClick={buyNow}
				disabled={busy || stock <= 0}
				className="h-12 rounded-md bg-action-500 px-6 font-semibold text-on-action hover:bg-action-600 disabled:opacity-50"
			>
				{t("storefront.product.buy_now")}
			</button>
			{/* Stays the form's first submit button: Enter in the quantity field adds to the cart. */}
			<button
				type="submit"
				disabled={busy || stock <= 0}
				className="h-12 rounded-md bg-action-soft px-6 font-semibold text-action-600 hover:bg-action-soft-hover disabled:opacity-50"
			>
				{busy ? t("cart.adding") : t("cart.add")}
			</button>
			{error && (
				<p role="alert" className="text-sm text-danger-600">
					{error}
				</p>
			)}
		</form>
	);
}
