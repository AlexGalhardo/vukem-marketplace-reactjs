import { Link, router, useForm } from "@inertiajs/react";
import { type FormEvent, type JSX, useId, useState } from "react";

import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { REPLY_MAX, replySchema } from "../../lib/reviews";
import type { ReviewsBlock, ReviewView } from "../../types";
import { ShieldIcon } from "../icons";
import { link, secondaryButton } from "../ui";
import Stars from "./Stars";

// oneDecimal rounds the stored ×100 average half up to one decimal, the
// way the server rounds (4.25 → 4.3), before Intl formats it.
export function oneDecimal(avgX100: number, locale: string): string {
	const value = Math.round(Math.max(0, Math.min(500, avgX100)) / 10) / 10;
	return value.toLocaleString(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
}

function Summary({ block }: { block: ReviewsBlock }): JSX.Element {
	const { t, locale } = useT();
	const { avg_x100: avg, count, distribution } = block.summary;
	const countLabel = t(count === 1 ? "reviews.count_one" : "reviews.count_other", { count });

	return (
		<div className="space-y-4">
			<div className="flex items-center gap-4">
				<p className="text-5xl font-normal text-action-600 tabular-nums">
					<span className="sr-only">{t("reviews.average", { avg: oneDecimal(avg, locale) })}</span>
					<span aria-hidden="true">{oneDecimal(avg, locale)}</span>
				</p>
				<div>
					<Stars rating={Math.round(avg / 100)} className="size-5" />
					<p className="mt-1 text-sm text-muted">{countLabel}</p>
				</div>
			</div>
			<ul aria-label={t("reviews.distribution")} className="space-y-1.5">
				{[5, 4, 3, 2, 1].map((stars) => {
					const n = distribution[stars - 1] ?? 0;
					const pct = count > 0 ? Math.round((n / count) * 100) : 0;
					return (
						<li key={stars} className="grid grid-cols-[4.5rem_1fr_2rem] items-center gap-2 text-sm">
							<span className="text-muted">
								{t(stars === 1 ? "reviews.stars_one" : "reviews.stars_other", { stars })}
							</span>
							<span aria-hidden="true" className="h-1.5 overflow-hidden rounded-full bg-sunken">
								<span className="block h-full rounded-full bg-muted" style={{ width: `${pct}%` }} />
							</span>
							<span className="text-right text-muted tabular-nums">{n}</span>
						</li>
					);
				})}
			</ul>
			{block.pending_item > 0 && (
				<Link href={`/order-items/${block.pending_item}/review`} className={`${secondaryButton} w-full`}>
					{t("reviews.write")}
				</Link>
			)}
			{block.list.total === 0 && <p className="text-sm text-muted">{t("reviews.none_hint")}</p>}
		</div>
	);
}

function ReplyForm({ review, max }: { review: ReviewView; max: number }): JSX.Element {
	const { t, locale } = useT();
	const id = useId();
	const form = useForm({ body: "" });
	const [clientError, setClientError] = useState<string>();
	const error = clientError ?? serverMessage(locale, form.errors.body);

	const submit = (e: FormEvent): void => {
		e.preventDefault();
		const parsed = replySchema({
			required: t("reviews.errors.reply_required"),
			tooLong: t("reviews.errors.too_long", { max }),
			contact: t("reviews.errors.contact"),
		}).safeParse(form.data.body);
		if (!parsed.success) {
			setClientError(parsed.error.issues[0]?.message);
			return;
		}
		setClientError(undefined);
		form.post(`/reviews/${review.id}/reply`, { preserveScroll: true, onSuccess: () => form.reset() });
	};

	return (
		<form onSubmit={submit} className="mt-3 space-y-2 border-l-2 border-line pl-3" noValidate>
			<label htmlFor={`${id}-reply`} className="block text-sm font-medium text-ink">
				{t("reviews.reply_label")}
			</label>
			<textarea
				id={`${id}-reply`}
				value={form.data.body}
				onChange={(e) => form.setData("body", e.target.value)}
				maxLength={max}
				rows={2}
				aria-invalid={error ? true : undefined}
				aria-describedby={`${id}-hint${error ? ` ${id}-error` : ""}`}
				className="w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus:border-action-500 focus:outline-none aria-invalid:border-danger-600"
			/>
			<p id={`${id}-hint`} className="text-xs text-muted">
				{t("reviews.reply_hint", { max })}
			</p>
			{error && (
				<p id={`${id}-error`} role="alert" className="text-sm text-danger-600">
					{error}
				</p>
			)}
			<button type="submit" disabled={form.processing} className={`${secondaryButton} h-9`}>
				{t("reviews.reply_submit")}
			</button>
		</form>
	);
}

function ReviewItem({ review, block }: { review: ReviewView; block: ReviewsBlock }): JSX.Element {
	const { t, locale } = useT();
	const date = new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(new Date(review.created_at));
	const helpfulText = t(review.helpful_count === 1 ? "reviews.helpful_count_one" : "reviews.helpful_count_other", {
		count: review.helpful_count,
	});

	const vote = (): void => {
		router.post(`/reviews/${review.id}/helpful`, {}, { preserveScroll: true });
	};

	return (
		<li className="border-t border-line py-4 first:border-t-0">
			<article aria-labelledby={`review-${review.id}`}>
				<div className="flex flex-wrap items-center gap-x-3 gap-y-1">
					<Stars rating={review.rating} />
					<span className="sr-only">{t("reviews.form.star", { n: review.rating })}</span>
					<h4 id={`review-${review.id}`} className="font-semibold break-words text-ink">
						{review.title || t("reviews.form.star", { n: review.rating })}
					</h4>
				</div>
				<p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
					<span>{t("reviews.by", { name: review.author, date })}</span>
					{review.verified && (
						<span className="inline-flex items-center gap-1 font-medium text-success-600">
							<ShieldIcon className="size-4" />
							{t("reviews.verified")}
						</span>
					)}
				</p>
				{review.body && (
					<p className="mt-2 max-w-prose break-words whitespace-pre-line text-ink">{review.body}</p>
				)}
				{review.recommend !== null && (
					<p className="mt-2 text-sm text-muted">
						{review.recommend ? t("reviews.recommend_yes") : t("reviews.recommend_no")}
					</p>
				)}
				{review.reply && (
					<div className="mt-3 rounded-md bg-sunken p-3 text-sm">
						<p className="font-semibold text-ink">{t("reviews.seller_reply")}</p>
						<p className="mt-1 break-words whitespace-pre-line text-ink">{review.reply.body}</p>
					</div>
				)}
				<div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
					{block.enabled && block.can_vote && !review.own && !review.voted && (
						<button
							type="button"
							onClick={vote}
							className="rounded-full border border-line px-3 py-1 text-action-600 hover:bg-action-soft"
						>
							{t("reviews.helpful")}
						</button>
					)}
					{review.voted && <span className="text-success-600">{t("reviews.helpful_voted")}</span>}
					{review.helpful_count > 0 && <span className="text-muted">{helpfulText}</span>}
				</div>
				{block.enabled && block.can_reply && !review.reply && (
					<ReplyForm review={review} max={block.max_reply} />
				)}
			</article>
		</li>
	);
}

// visit reloads only the reviews prop with new list parameters.
function visit(params: Record<string, string | number>): void {
	const url = new URL(window.location.href);
	for (const [key, value] of Object.entries(params)) url.searchParams.set(key, String(value));
	router.get(url.pathname + url.search, {}, { only: ["reviews"], preserveScroll: true, preserveState: true });
}

// ReviewsSection is the reviews block of the product page: average,
// distribution, the write link for an eligible buyer, sort, the list with
// votes and the shop's replies, pagination.
export default function ReviewsSection({ block }: { block: ReviewsBlock }): JSX.Element {
	const { t } = useT();
	const { list } = block;
	const sortId = useId();
	const max = block.max_reply || REPLY_MAX;

	return (
		<section id="reviews" aria-labelledby="reviews-title" className="scroll-mt-4">
			<h2 id="reviews-title" className="mb-3 text-xl font-semibold text-ink">
				{t("reviews.title")}
			</h2>
			{block.summary.count === 0 && list.total === 0 ? (
				<div className="space-y-3">
					<p className="text-muted">{t("reviews.none")}</p>
					{block.pending_item > 0 && (
						<Link href={`/order-items/${block.pending_item}/review`} className={secondaryButton}>
							{t("reviews.write")}
						</Link>
					)}
				</div>
			) : (
				<div className="grid gap-6 md:grid-cols-[16rem_minmax(0,1fr)]">
					<Summary block={block} />
					<div>
						<div className="flex flex-wrap items-center justify-between gap-2">
							<h3 className="font-semibold text-ink">{t("reviews.list")}</h3>
							<div className="flex items-center gap-2 text-sm">
								<label htmlFor={sortId} className="text-muted">
									{t("reviews.sort")}
								</label>
								<select
									id={sortId}
									value={list.sort}
									onChange={(e) => visit({ rsort: e.target.value, rpage: 1 })}
									className="h-9 rounded-md border border-line bg-card px-2 text-sm text-ink"
								>
									<option value="newest">{t("reviews.sort_newest")}</option>
									<option value="helpful">{t("reviews.sort_helpful")}</option>
								</select>
							</div>
						</div>
						<ul className="mt-2">
							{list.data.map((review) => (
								<ReviewItem key={review.id} review={review} block={{ ...block, max_reply: max }} />
							))}
						</ul>
						{list.last_page > 1 && (
							<nav
								aria-label={t("catalog.pagination.label")}
								className="mt-4 flex items-center justify-center gap-2 text-sm"
							>
								{list.page > 1 && (
									<button
										type="button"
										className={link}
										onClick={() => visit({ rpage: list.page - 1 })}
									>
										{t("catalog.pagination.previous")}
									</button>
								)}
								<span className="px-2 text-muted">
									{t("catalog.pagination.status", { page: list.page, last: list.last_page })}
								</span>
								{list.page < list.last_page && (
									<button
										type="button"
										className={link}
										onClick={() => visit({ rpage: list.page + 1 })}
									>
										{t("catalog.pagination.next")}
									</button>
								)}
							</nav>
						)}
					</div>
				</div>
			)}
		</section>
	);
}
