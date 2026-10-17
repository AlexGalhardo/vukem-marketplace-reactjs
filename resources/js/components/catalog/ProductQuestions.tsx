import { Field as BaseField } from "@base-ui/react/field";
import { Link, useForm, usePage } from "@inertiajs/react";
import { type FormEvent, type JSX, type ReactNode, useMemo, useState } from "react";
import { useT } from "../../lib/i18n";
import { check, QUESTION_MAX, QUESTION_MIN, textSchema } from "../../lib/qna";
import type { Page, PublicQuestion, QnaMeta, SharedProps } from "../../types";
import { ChatIcon } from "../icons";
import { button, link } from "../ui";

interface Props {
	slug: string;
	questions: Page<PublicQuestion>;
	qna: QnaMeta;
}

// ProductQuestions is the questions and answers section of the product
// page: the ask form (signed-in buyers), then the questions, newest first.
export default function ProductQuestions({ slug, questions, qna }: Props): JSX.Element {
	const { t } = useT();

	return (
		<section aria-labelledby="questions-title">
			<h2 id="questions-title" className="mb-3 text-xl font-semibold text-ink">
				{t("qna.title")}
			</h2>
			<AskBox slug={slug} qna={qna} />
			<QuestionList slug={slug} questions={questions} />
		</section>
	);
}

function Notice({ children }: { children: ReactNode }): JSX.Element {
	return (
		<p className="mb-6 flex items-start gap-2 rounded-md bg-sunken p-4 text-sm text-muted">
			<ChatIcon className="size-5 shrink-0 text-action-600" />
			<span>{children}</span>
		</p>
	);
}

function AskBox({ slug, qna }: { slug: string; qna: QnaMeta }): JSX.Element {
	const { t } = useT();
	const user = usePage<SharedProps>().props.auth?.user;

	if (qna.own_product) {
		return (
			<Notice>
				{t("qna.own_product")}{" "}
				<Link href="/seller/questions" className={link}>
					{t("qna.own_product_link")}
				</Link>
			</Notice>
		);
	}

	if (!qna.enabled) return <Notice>{t("qna.disabled")}</Notice>;

	if (!user) {
		return (
			<Notice>
				<Link href="/login" className={link}>
					{t("qna.login_to_ask")}
				</Link>
			</Notice>
		);
	}

	return <AskForm slug={slug} />;
}

function AskForm({ slug }: { slug: string }): JSX.Element {
	const { t } = useT();
	const form = useForm({ body: "" });
	const [sent, setSent] = useState(false);
	const schema = useMemo(
		() =>
			textSchema(QUESTION_MIN, QUESTION_MAX, {
				tooShort: t("qna.errors.too_short", { min: QUESTION_MIN }),
				tooLong: t("qna.errors.too_long", { max: QUESTION_MAX }),
				contact: t("qna.errors.no_contact"),
			}),
		[t],
	);

	const submit = (e: FormEvent) => {
		e.preventDefault();
		setSent(false);
		const problem = check(schema, form.data.body);
		if (problem) {
			form.setError("body", problem);
			return;
		}
		form.clearErrors();
		form.post(`/p/${slug}/questions`, {
			preserveScroll: true,
			onSuccess: () => {
				form.reset();
				setSent(true);
			},
			onHttpException: (response) => {
				if (response.status !== 429) return;
				form.setError("body", t("qna.errors.rate_limited"));
				return false;
			},
		});
	};

	const count = Array.from(form.data.body.trim()).length;

	return (
		<form onSubmit={submit} noValidate className="mb-6 flex flex-col gap-2">
			<BaseField.Root invalid={Boolean(form.errors.body)} className="flex flex-col gap-1">
				<BaseField.Label className="text-sm font-medium text-ink">{t("qna.ask_label")}</BaseField.Label>
				<BaseField.Control
					render={<textarea rows={3} />}
					name="body"
					maxLength={QUESTION_MAX + 50}
					value={form.data.body}
					onChange={(e) => form.setData("body", e.currentTarget.value)}
					className="rounded-md border border-line bg-card px-3 py-2 text-sm text-ink placeholder:text-muted focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
				/>
				<div className="flex items-start justify-between gap-4">
					<BaseField.Description className="text-xs text-muted">
						{t("qna.ask_hint", { min: QUESTION_MIN, max: QUESTION_MAX })}
					</BaseField.Description>
					<span
						aria-hidden="true"
						className={`shrink-0 text-xs tabular-nums ${count > QUESTION_MAX ? "text-danger-600" : "text-muted"}`}
					>
						{t("qna.chars", { count, max: QUESTION_MAX })}
					</span>
				</div>
				<BaseField.Error match={true} className="text-sm text-danger-600">
					{form.errors.body}
				</BaseField.Error>
			</BaseField.Root>
			<div className="flex flex-wrap items-center gap-3">
				<button type="submit" disabled={form.processing} className={button}>
					{form.processing ? t("qna.asking") : t("qna.ask_submit")}
				</button>
				<p role="status" className="text-sm text-success-600">
					{sent ? t("qna.flash.asked") : ""}
				</p>
			</div>
		</form>
	);
}

function QuestionList({ slug, questions }: { slug: string; questions: Page<PublicQuestion> }): JSX.Element {
	const { t, locale } = useT();
	const date = new Intl.DateTimeFormat(locale, { dateStyle: "medium" });

	if (questions.total === 0) return <p className="text-muted">{t("qna.empty")}</p>;

	return (
		<>
			<p className="mb-3 text-sm text-muted">{t("qna.count", { count: questions.total })}</p>
			<ul className="divide-y divide-line" data-testid="question-list">
				{questions.data.map((q) => (
					<li key={q.id} className="py-4">
						<p className="break-words whitespace-pre-line text-ink">{q.body}</p>
						<p className="mt-1 text-xs text-muted">
							{t("qna.asked_by", { name: q.asker_name })} ·{" "}
							<time dateTime={q.created_at}>{date.format(new Date(q.created_at))}</time>
						</p>
						{q.answered_at ? (
							<div className="mt-3 border-l-2 border-action-500 pl-3">
								<p className="text-xs font-medium text-muted">{t("qna.answer_label")}</p>
								<p className="mt-0.5 break-words whitespace-pre-line text-ink">{q.answer}</p>
								<p className="mt-1 text-xs text-muted">
									<time dateTime={q.answered_at}>{date.format(new Date(q.answered_at))}</time>
								</p>
							</div>
						) : (
							<p className="mt-2 text-sm text-muted italic">{t("qna.awaiting")}</p>
						)}
					</li>
				))}
			</ul>
			{questions.last_page > 1 && (
				<nav aria-label={t("qna.pagination")} className="mt-4 flex items-center justify-between gap-3 text-sm">
					<PageLink slug={slug} page={questions.page - 1} disabled={questions.page <= 1}>
						{t("qna.previous")}
					</PageLink>
					<span className="text-muted">
						{t("qna.page_of", { page: questions.page, last: questions.last_page })}
					</span>
					<PageLink slug={slug} page={questions.page + 1} disabled={questions.page >= questions.last_page}>
						{t("qna.next")}
					</PageLink>
				</nav>
			)}
		</>
	);
}

function PageLink({
	slug,
	page,
	disabled,
	children,
}: {
	slug: string;
	page: number;
	disabled: boolean;
	children: string;
}): JSX.Element {
	if (disabled) return <span className="text-muted">{children}</span>;

	return (
		<Link href={`/p/${slug}?q_page=${page}`} only={["questions"]} preserveScroll preserveState className={link}>
			{children}
		</Link>
	);
}
