import { Field as BaseField } from "@base-ui/react/field";
import { Head, Link, router, useForm } from "@inertiajs/react";
import { type FormEvent, type JSX, useCallback, useMemo, useState } from "react";

import SellerNav from "../../components/catalog/SellerNav";
import { button, link } from "../../components/ui";
import { useT } from "../../lib/i18n";
import { ANSWER_MAX, check, textSchema } from "../../lib/qna";
import { useEventStream } from "../../lib/useEventStream";
import type { Page, SellerQuestion } from "../../types";

interface Props {
	filter: "unanswered" | "answered";
	questions: Page<SellerQuestion>;
	unanswered: number;
}

interface Notification {
	type?: string;
}

export default function Questions({ filter, questions, unanswered }: Props): JSX.Element {
	const { t } = useT();
	const [live, setLive] = useState("");

	// A new question arrives over SSE: reload the list in place.
	const onEvent = useCallback(
		(payload: Notification) => {
			if (payload.type !== "qna.question") return;
			router.reload({ only: ["questions", "unanswered"] });
			setLive(t("qna.seller.live"));
		},
		[t],
	);
	useEventStream<Notification>("notifications", onEvent);

	const tabs = [
		{ key: "unanswered", href: "/seller/questions", label: t("qna.seller.unanswered", { count: unanswered }) },
		{ key: "answered", href: "/seller/questions?filter=answered", label: t("qna.seller.answered") },
	] as const;

	return (
		<>
			<Head title={t("qna.seller.title")} />
			<SellerNav current="/seller/questions" />
			<h1 className="text-2xl font-semibold text-ink">{t("qna.seller.title")}</h1>
			<p className="mt-1 text-sm text-muted">{t("qna.seller.intro")}</p>
			<p role="status" className="sr-only">
				{live}
			</p>

			<nav aria-label={t("qna.seller.filters")} className="mt-5 flex gap-2">
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
					{filter === "answered" ? t("qna.seller.empty_answered") : t("qna.seller.empty_unanswered")}
				</p>
			) : (
				<ul className="mt-6 space-y-4">
					{questions.data.map((q) => (
						<QuestionCard key={q.id} question={q} />
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
	if (filter === "answered") params.set("filter", "answered");
	return `/seller/questions?${params.toString()}`;
}

function QuestionCard({ question }: { question: SellerQuestion }): JSX.Element {
	const { t, locale } = useT();
	const form = useForm({ answer: question.answer });
	const fieldId = `answer-${question.id}`;
	const schema = useMemo(
		() =>
			textSchema(1, ANSWER_MAX, {
				tooShort: t("qna.errors.too_short", { min: 1 }),
				tooLong: t("qna.errors.too_long", { max: ANSWER_MAX }),
				contact: t("qna.errors.no_contact"),
			}),
		[t],
	);
	const date = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		const problem = check(schema, form.data.answer);
		if (problem) {
			form.setError("answer", problem);
			return;
		}
		form.clearErrors();
		form.post(`/seller/questions/${question.id}/answer`, { preserveScroll: true });
	};

	return (
		<li className="rounded-md bg-card p-4 shadow-card" aria-labelledby={`q-${question.id}`}>
			<p className="text-xs text-muted">
				<Link href={`/p/${question.product.slug}`} className={link}>
					{t("qna.seller.about", { product: question.product.title })}
				</Link>
			</p>
			<p id={`q-${question.id}`} className="mt-1 font-medium break-words whitespace-pre-line text-ink">
				{question.body}
			</p>
			<p className="mt-1 text-xs text-muted">
				{t("qna.asked_by", { name: question.asker_name })} ·{" "}
				<time dateTime={question.created_at}>{date.format(new Date(question.created_at))}</time>
			</p>
			<form onSubmit={submit} noValidate className="mt-3 flex flex-col gap-2">
				<BaseField.Root invalid={Boolean(form.errors.answer)} className="flex flex-col gap-1">
					<BaseField.Label htmlFor={fieldId} className="text-sm font-medium text-ink">
						{t("qna.seller.answer_label")}
					</BaseField.Label>
					<BaseField.Control
						id={fieldId}
						render={<textarea rows={3} />}
						name="answer"
						maxLength={ANSWER_MAX + 50}
						value={form.data.answer}
						onChange={(e) => form.setData("answer", e.currentTarget.value)}
						className="rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
					/>
					<BaseField.Description className="text-xs text-muted">
						{t("qna.seller.answer_hint", { max: ANSWER_MAX })}
					</BaseField.Description>
					<BaseField.Error match={true} className="text-sm text-danger-600">
						{form.errors.answer}
					</BaseField.Error>
				</BaseField.Root>
				<div>
					<button type="submit" disabled={form.processing} className={button}>
						{form.processing
							? t("qna.seller.answering")
							: question.answered_at
								? t("qna.seller.edit_submit")
								: t("qna.seller.answer_submit")}
					</button>
				</div>
			</form>
		</li>
	);
}
