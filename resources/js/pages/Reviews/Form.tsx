import { Head, Link, useForm } from "@inertiajs/react";
import { type FormEvent, type JSX, useState } from "react";

import Field from "../../components/Field";
import { button, link, panel } from "../../components/ui";
import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { fieldErrors, length, type ReviewFields, reviewSchema } from "../../lib/reviews";
import type { ReviewItem } from "../../types";

interface Props {
	item: ReviewItem;
	limits: { title: number; body: number };
}

const star = "m10 1.8 2.5 5.2 5.7.8-4.1 4 1 5.7L10 14.8l-5.1 2.7 1-5.7-4.1-4 5.7-.8z";

export default function ReviewForm({ item, limits }: Props): JSX.Element {
	const { t, locale } = useT();
	const form = useForm<ReviewFields>({ rating: 0, title: "", body: "", recommend: "" });
	const [clientErrors, setClientErrors] = useState<Record<string, string>>({});

	const error = (field: keyof ReviewFields): string | undefined =>
		clientErrors[field] ?? serverMessage(locale, form.errors[field]);

	const submit = (e: FormEvent): void => {
		e.preventDefault();
		const parsed = reviewSchema({
			rating: t("reviews.form.rating_required"),
			tooLong: (max) => t("reviews.errors.too_long", { max }),
			contact: t("reviews.errors.contact"),
		}).safeParse(form.data);
		if (!parsed.success) {
			setClientErrors(fieldErrors(parsed.error));
			return;
		}
		setClientErrors({});
		form.post(`/order-items/${item.order_item_id}/review`);
	};

	const ratingError = error("rating");
	const bodyError = error("body");
	const recommendOptions = [
		{ value: "yes", label: t("reviews.form.recommend_yes") },
		{ value: "no", label: t("reviews.form.recommend_no") },
		{ value: "", label: t("reviews.form.recommend_skip") },
	] as const;

	return (
		<>
			<Head title={t("reviews.form.title")} />
			<div className={`${panel} mx-auto max-w-2xl`}>
				<h1 className="text-2xl font-semibold text-ink">{t("reviews.form.title")}</h1>
				<p className="mt-1 break-words text-muted">
					{t("reviews.form.intro", { product: item.product_title })}
				</p>

				<form onSubmit={submit} noValidate className="mt-6 space-y-6">
					<fieldset aria-describedby={ratingError ? "rating-error" : undefined}>
						<legend className="text-sm font-medium text-ink">{t("reviews.form.rating")}</legend>
						<div className="mt-2 flex gap-1">
							{[1, 2, 3, 4, 5].map((n) => (
								<label key={n} className="cursor-pointer">
									<input
										type="radio"
										name="rating"
										value={n}
										checked={form.data.rating === n}
										onChange={() => form.setData("rating", n)}
										className="peer sr-only"
									/>
									<span className="sr-only">{t("reviews.form.star", { n })}</span>
									<svg
										viewBox="0 0 20 20"
										aria-hidden="true"
										className="size-9 rounded-md text-star peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-action-500"
									>
										<path
											d={star}
											fill="currentColor"
											fillOpacity={n <= form.data.rating ? 1 : 0.25}
										/>
									</svg>
								</label>
							))}
						</div>
						{ratingError && (
							<p id="rating-error" role="alert" className="mt-1 text-sm text-danger-600">
								{ratingError}
							</p>
						)}
					</fieldset>

					<Field
						label={t("reviews.form.title_label")}
						maxLength={limits.title}
						value={form.data.title}
						error={error("title")}
						onChange={(e) => form.setData("title", e.target.value)}
					/>

					<div className="flex flex-col gap-1">
						<label htmlFor="review-body" className="text-sm font-medium text-ink">
							{t("reviews.form.body_label")}
						</label>
						<textarea
							id="review-body"
							rows={6}
							maxLength={limits.body}
							value={form.data.body}
							onChange={(e) => form.setData("body", e.target.value)}
							aria-invalid={bodyError ? true : undefined}
							aria-describedby={`review-body-count${bodyError ? " review-body-error" : ""}`}
							className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus:border-action-500 focus:outline-none aria-invalid:border-danger-600"
						/>
						<p id="review-body-count" className="text-right text-xs text-muted tabular-nums">
							{t("reviews.form.counter", { count: length(form.data.body), max: limits.body })}
						</p>
						{bodyError && (
							<p id="review-body-error" role="alert" className="text-sm text-danger-600">
								{bodyError}
							</p>
						)}
					</div>

					<fieldset>
						<legend className="text-sm font-medium text-ink">{t("reviews.form.recommend")}</legend>
						<div className="mt-2 flex flex-wrap gap-4">
							{recommendOptions.map((o) => (
								<label key={o.value} className="flex items-center gap-2 text-sm text-ink">
									<input
										type="radio"
										name="recommend"
										value={o.value}
										checked={form.data.recommend === o.value}
										onChange={() => form.setData("recommend", o.value)}
										className="size-4 accent-action-500"
									/>
									{o.label}
								</label>
							))}
						</div>
						{error("recommend") && (
							<p role="alert" className="mt-1 text-sm text-danger-600">
								{error("recommend")}
							</p>
						)}
					</fieldset>

					<p className="text-sm text-muted">{t("reviews.form.rules")}</p>

					<div className="flex flex-wrap items-center gap-4">
						<button type="submit" disabled={form.processing} className={button}>
							{t("reviews.form.submit")}
						</button>
						<Link href={`/p/${item.product_slug}`} className={link}>
							{t("reviews.form.back")}
						</Link>
					</div>
				</form>
			</div>
		</>
	);
}
