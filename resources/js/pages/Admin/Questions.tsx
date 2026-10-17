import { Field as BaseField } from "@base-ui/react/field";
import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import { link, secondaryButton } from "../../components/ui";
import { useT } from "../../lib/i18n";
import { REASON_MAX, REASON_MIN } from "../../lib/qna";
import type { AdminQuestion, Page } from "../../types";

interface Props {
	filter: "all" | "hidden";
	questions: Page<AdminQuestion>;
}

export default function Questions({ filter, questions }: Props): JSX.Element {
	const { t } = useT();
	const tabs = [
		{ key: "all", href: "/admin/questions", label: t("qna.admin.all") },
		{ key: "hidden", href: "/admin/questions?filter=hidden", label: t("qna.admin.hidden") },
	] as const;

	return (
		<>
			<Head title={t("qna.admin.title")} />
			<h1 className="text-2xl font-semibold text-ink">{t("qna.admin.title")}</h1>
			<p className="mt-1 max-w-prose text-sm text-muted">{t("qna.admin.intro")}</p>

			<nav aria-label={t("qna.admin.filters")} className="mt-5 flex gap-2">
				{tabs.map((tab) => (
					<Link
						key={tab.key}
						href={tab.href}
						aria-current={tab.key === filter ? "page" : undefined}
						className="rounded-full border border-line px-3 py-1.5 text-sm text-ink hover:border-action-500 aria-[current=page]:border-action-500 aria-[current=page]:bg-action-soft aria-[current=page]:text-action-600"
					>
						{tab.label}
					</Link>
				))}
			</nav>

			{questions.data.length === 0 ? (
				<p className="mt-6 rounded-md bg-card px-4 py-6 text-sm text-muted shadow-card">
					{t("qna.admin.empty")}
				</p>
			) : (
				<ul className="mt-6 space-y-4">
					{questions.data.map((q) => (
						<Row key={q.id} question={q} filter={filter} />
					))}
				</ul>
			)}

			{questions.last_page > 1 && (
				<nav aria-label={t("qna.pagination")} className="mt-6 flex items-center justify-between text-sm">
					{questions.page > 1 ? (
						<Link href={pageHref(filter, questions.page - 1)} className={link}>
							{t("qna.previous")}
						</Link>
					) : (
						<span />
					)}
					<span className="text-muted">
						{t("qna.page_of", { page: questions.page, last: questions.last_page })}
					</span>
					{questions.page < questions.last_page ? (
						<Link href={pageHref(filter, questions.page + 1)} className={link}>
							{t("qna.next")}
						</Link>
					) : (
						<span />
					)}
				</nav>
			)}
		</>
	);
}

function pageHref(filter: string, page: number): string {
	const params = new URLSearchParams({ page: String(page) });
	if (filter === "hidden") params.set("filter", "hidden");
	return `/admin/questions?${params.toString()}`;
}

function Row({ question, filter }: { question: AdminQuestion; filter: string }): JSX.Element {
	const { t } = useT();
	const form = useForm({ reason: "" });
	const suffix = filter === "hidden" ? "?filter=hidden" : "";
	const fieldId = `reason-${question.id}`;

	const hide = (e: FormEvent) => {
		e.preventDefault();
		form.post(`/admin/questions/${question.id}/hide${suffix}`, { preserveScroll: true });
	};

	const unhide = (e: FormEvent) => {
		e.preventDefault();
		form.post(`/admin/questions/${question.id}/unhide${suffix}`, { preserveScroll: true });
	};

	return (
		<li className="rounded-md bg-card p-4 shadow-card">
			<div className="flex flex-wrap items-center gap-2 text-xs text-muted">
				<Link href={`/p/${question.product.slug}`} className={link}>
					{question.product.title}
				</Link>
				<span>·</span>
				<span>{t("qna.admin.shop", { shop: question.shop_name })}</span>
				{question.hidden_at && (
					<span className="rounded-full bg-danger-600/10 px-2 py-0.5 font-medium text-danger-600">
						{t("qna.admin.hidden_badge")}
					</span>
				)}
			</div>
			<p className="mt-2 break-words whitespace-pre-line text-ink">{question.body}</p>
			<p className="mt-1 text-xs text-muted">
				{t("qna.admin.asker", { name: question.asker_name, id: question.asker_id })}
			</p>
			{question.answered_at && (
				<p className="mt-2 border-l-2 border-line pl-3 text-sm break-words whitespace-pre-line text-muted">
					{question.answer}
				</p>
			)}
			{question.hidden_at ? (
				<form onSubmit={unhide} className="mt-3 flex flex-wrap items-center gap-3">
					<p className="text-sm text-muted">
						{t("qna.admin.hidden_reason", { reason: question.hidden_reason })}
					</p>
					<button type="submit" disabled={form.processing} className={secondaryButton}>
						{t("qna.admin.unhide")}
					</button>
				</form>
			) : (
				<form onSubmit={hide} className="mt-3 flex flex-wrap items-end gap-3">
					<BaseField.Root
						invalid={Boolean(form.errors.reason)}
						className="flex min-w-60 flex-1 flex-col gap-1"
					>
						<BaseField.Label htmlFor={fieldId} className="text-sm font-medium text-ink">
							{t("qna.admin.reason_label")}
						</BaseField.Label>
						<BaseField.Control
							id={fieldId}
							required
							minLength={REASON_MIN}
							maxLength={REASON_MAX}
							value={form.data.reason}
							onChange={(e) => form.setData("reason", e.currentTarget.value)}
							className="h-10 rounded-md border border-line bg-card px-3 text-sm text-ink focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
						/>
						<BaseField.Error match={true} className="text-sm text-danger-600">
							{form.errors.reason}
						</BaseField.Error>
					</BaseField.Root>
					<button type="submit" disabled={form.processing} className={secondaryButton}>
						{t("qna.admin.hide")}
					</button>
				</form>
			)}
		</li>
	);
}
