import { Head, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import PasswordField from "../../components/PasswordField";
import { button, card, heading } from "../../components/ui";
import { PASSWORD_MIN } from "../../lib/credentials";
import { useT } from "../../lib/i18n";

export default function ResetPassword({ token, email }: { token: string; email: string }): JSX.Element {
	const { t } = useT();
	const form = useForm({ token, email, password: "", password_confirmation: "" });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/reset-password", { onFinish: () => form.reset("password", "password_confirmation") });
	};

	return (
		<section className={card}>
			<Head title={t("account.reset.title")} />
			<h1 className={heading}>{t("account.reset.title")}</h1>
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
				<PasswordField
					label={t("account.common.new_password")}
					requirements
					autoComplete="new-password"
					minLength={PASSWORD_MIN}
					required
					value={form.data.password}
					onChange={(e) => form.setData("password", e.target.value)}
					error={form.errors.password}
				/>
				<PasswordField
					label={t("account.common.confirm_password")}
					autoComplete="new-password"
					required
					value={form.data.password_confirmation}
					onChange={(e) => form.setData("password_confirmation", e.target.value)}
					error={form.errors.password_confirmation}
				/>
				<button type="submit" disabled={form.processing} className={button}>
					{t("account.reset.submit")}
				</button>
			</form>
		</section>
	);
}
