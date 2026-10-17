import { Field as BaseField } from "@base-ui/react/field";
import { Head, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../components/Field";
import { useT } from "../lib/i18n";

export default function Contact(): JSX.Element {
	const { t } = useT();
	const form = useForm({ name: "", email: "", subject: "", body: "" });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/contact", { preserveScroll: true, onSuccess: () => form.reset() });
	};

	return (
		<>
			<Head title={t("contact.title")} />
			<div className="mx-auto max-w-2xl rounded-lg bg-card px-6 py-8 shadow-sm">
				<h1 className="text-2xl font-semibold text-ink">{t("contact.title")}</h1>
				<p className="mt-2 max-w-prose text-muted">{t("contact.intro")}</p>
				<form onSubmit={submit} className="mt-6 grid gap-4 sm:grid-cols-2">
					<Field
						label={t("contact.name")}
						autoComplete="name"
						required
						maxLength={100}
						value={form.data.name}
						onChange={(e) => form.setData("name", e.target.value)}
						error={form.errors.name}
					/>
					<Field
						label={t("contact.email")}
						type="email"
						autoComplete="email"
						required
						maxLength={255}
						value={form.data.email}
						onChange={(e) => form.setData("email", e.target.value)}
						error={form.errors.email}
					/>
					<div className="sm:col-span-2">
						<Field
							label={t("contact.subject")}
							required
							maxLength={150}
							value={form.data.subject}
							onChange={(e) => form.setData("subject", e.target.value)}
							error={form.errors.subject}
						/>
					</div>
					<BaseField.Root invalid={Boolean(form.errors.body)} className="flex flex-col gap-1 sm:col-span-2">
						<BaseField.Label className="text-sm font-medium text-ink">{t("contact.body")}</BaseField.Label>
						<BaseField.Control
							render={<textarea rows={7} />}
							required
							minLength={10}
							maxLength={5000}
							value={form.data.body}
							onChange={(e) => form.setData("body", e.currentTarget.value)}
							className="rounded-md border border-line bg-card px-3 py-2 text-sm text-ink placeholder:text-muted focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
						/>
						<BaseField.Description className="text-xs text-muted">
							{t("contact.body_hint")}
						</BaseField.Description>
						<BaseField.Error match={true} className="text-sm text-danger-600">
							{form.errors.body}
						</BaseField.Error>
					</BaseField.Root>
					<div className="sm:col-span-2">
						<button
							type="submit"
							disabled={form.processing}
							className="rounded-md bg-action-600 px-5 py-2.5 font-medium text-white hover:bg-action-700 disabled:opacity-50"
						>
							{form.processing ? t("contact.sending") : t("contact.submit")}
						</button>
					</div>
				</form>
			</div>
		</>
	);
}
