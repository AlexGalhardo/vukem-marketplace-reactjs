import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import { button, card, heading, link, muted } from "../../components/ui";
import { useT } from "../../lib/i18n";

export default function ForgotPassword(): JSX.Element {
	const { t } = useT();
	const form = useForm({ email: "" });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/forgot-password");
	};

	return (
		<section className={card}>
			<Head title={t("account.forgot.title")} />
			<h1 className={heading}>{t("account.forgot.title")}</h1>
			<p className={`mb-4 ${muted}`}>{t("account.forgot.intro")}</p>
			<form onSubmit={submit} className="space-y-4">
				<Field
					label={t("account.common.email")}
					type="email"
					autoComplete="email"
					required
					value={form.data.email}
					onChange={(e) => form.setData("email", e.target.value)}
					error={form.errors.email}
				/>
				<button type="submit" disabled={form.processing} className={button}>
					{t("account.forgot.submit")}
				</button>
			</form>
			<p className="mt-6 text-sm">
				<Link href="/login" className={link}>
					{t("account.login.title")}
				</Link>
			</p>
		</section>
	);
}
