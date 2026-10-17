import { Head, Link, router, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Stars from "../../components/reviews/Stars";
import { dangerButton, link, secondaryButton } from "../../components/ui";
import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import { formatDateTime } from "../../lib/money";
import type { AdminReview, Page } from "../../types";

interface Props {
	reviews: Page<AdminReview> & { filter: "all" | "hidden" };
	limits: { reasonMin: number; reasonMax: number };
}

function Row({ review, limits }: { review: AdminReview; limits: Props["limits"] }): JSX.Element {
	const { t, locale } = useT();
	const hide = useForm({ reason: "" });
	const reasonId = `reason-${review.id}`;
	const reasonError = serverMessage(locale, hide.errors.reason);

	const submitHide = (e: FormEvent): void => {
		e.preventDefault();
		hide.post(`/admin/reviews/${review.id}/hide`, { preserveScroll: true, onSuccess: () => hide.reset() });
	};

	const unhide = (): void => {
		router.post(`/admin/reviews/${review.id}/unhide`, {}, { preserveScroll: true });
	};

	return (
		<li className="grid gap-3 px-4 py-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
			<div className="min-w-0">
				<div className="flex flex-wrap items-center gap-2">
					<Stars rating={review.rating} />
					<span className="sr-only">{t("reviews.form.star", { n: review.rating })}</span>
					{review.hidden_at && (
						<span className="rounded-full bg-sunken px-2 py-0.5 text-xs font-semibold text-danger-600">
							{t("reviews.admin.hidden_badge")}
						</span>
					)}
				</div>
				{review.title && <p className="mt-1 font-semibold break-words">{review.title}</p>}
				{review.body && <p className="mt-1 break-words whitespace-pre-line text-sm">{review.body}</p>}
				<p className="mt-1 text-xs text-muted">
					{review.author} · {formatDateTime(review.created_at, locale)} · {t("reviews.admin.product")}:{" "}
					<Link href={`/p/${review.product_slug}`} className={`${link} underline`}>
						{review.product_title}
					</Link>
				</p>
				{review.hidden_at && (
					<p className="mt-1 text-sm text-muted">
						{t("reviews.admin.hidden_reason", { reason: review.hidden_reason })}
					</p>
				)}
			</div>
			{review.hidden_at ? (
				<div>
					<button type="button" onClick={unhide} className={secondaryButton}>
						{t("reviews.admin.unhide")}
					</button>
				</div>
			) : (
				<form onSubmit={submitHide} className="flex flex-col gap-1">
					<label htmlFor={reasonId} className="text-sm">
						{t("reviews.admin.reason")}
					</label>
					<div className="flex gap-2">
						<input
							id={reasonId}
							required
							minLength={limits.reasonMin}
							maxLength={limits.reasonMax}
							value={hide.data.reason}
							onChange={(e) => hide.setData("reason", e.target.value)}
							aria-invalid={reasonError ? true : undefined}
							aria-describedby={reasonError ? `${reasonId}-error` : undefined}
							className="h-10 min-w-0 flex-1 rounded-md border border-line bg-card px-3 text-sm text-ink"
						/>
						<button type="submit" disabled={hide.processing} className={`${dangerButton} h-10`}>
							{t("reviews.admin.hide")}
						</button>
					</div>
					{reasonError && (
						<p id={`${reasonId}-error`} role="alert" className="text-sm text-danger-600">
							{reasonError}
						</p>
					)}
				</form>
			)}
		</li>
	);
}

export default function AdminReviews({ reviews, limits }: Props): JSX.Element {
	const { t } = useT();
	const filters = [
		{ value: "all", label: t("reviews.admin.filter_all") },
		{ value: "hidden", label: t("reviews.admin.filter_hidden") },
	] as const;
	const pageHref = (page: number): string => `/admin/reviews?filter=${reviews.filter}&page=${page}`;

	return (
		<>
			<Head title={t("reviews.admin.title")} />
			<h1 className="text-2xl font-semibold text-ink">{t("reviews.admin.title")}</h1>
			<p className="mt-2 max-w-prose text-muted">{t("reviews.admin.intro")}</p>

			<nav aria-label={t("reviews.admin.filter")} className="mt-4 flex gap-2">
				{filters.map((f) => (
					<Link
						key={f.value}
						href={`/admin/reviews?filter=${f.value}`}
						aria-current={reviews.filter === f.value ? "page" : undefined}
						className="rounded-full border border-line px-3 py-1 text-sm text-action-600 aria-[current=page]:bg-action-soft aria-[current=page]:font-semibold"
					>
						{f.label}
					</Link>
				))}
			</nav>

			<section className="mt-4 rounded-md bg-card text-ink shadow-card">
				{reviews.data.length === 0 ? (
					<p className="px-4 py-6 text-muted">{t("reviews.admin.empty")}</p>
				) : (
					<ul className="divide-y divide-line">
						{reviews.data.map((r) => (
							<Row key={r.id} review={r} limits={limits} />
						))}
					</ul>
				)}
			</section>

			{reviews.last_page > 1 && (
				<nav aria-label={t("catalog.pagination.label")} className="mt-4 flex items-center justify-center gap-2">
					{reviews.page > 1 && (
						<Link href={pageHref(reviews.page - 1)} className={link}>
							{t("catalog.pagination.previous")}
						</Link>
					)}
					<span className="px-2 text-sm text-muted">
						{t("catalog.pagination.status", { page: reviews.page, last: reviews.last_page })}
					</span>
					{reviews.page < reviews.last_page && (
						<Link href={pageHref(reviews.page + 1)} className={link}>
							{t("catalog.pagination.next")}
						</Link>
					)}
				</nav>
			)}
		</>
	);
}
