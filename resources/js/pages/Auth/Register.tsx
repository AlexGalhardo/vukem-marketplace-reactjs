import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import PasswordField from "../../components/PasswordField";
import SocialButtons from "../../components/SocialButtons";
import { button, card, heading, link } from "../../components/ui";
import { EMAIL_MAX, EMAIL_MIN, PASSWORD_MIN } from "../../lib/credentials";
import { useT } from "../../lib/i18n";
import { capitalizeName, NAME_MAX, NAME_MIN, parseName } from "../../lib/name";
import type { AuthPageProps } from "../../types";

export default function Register({ socialProviders = [] }: AuthPageProps): JSX.Element {
	const { t } = useT();
	const form = useForm({ name: "", email: "", password: "", password_confirmation: "" });

	// checkName shows the name rule's message, or returns the normalized name.
	const checkName = (): string | null => {
		const result = parseName(form.data.name, t("account.errors.name_length"));
		if ("error" in result) {
			form.setError("name", result.error);
			return null;
		}
		form.clearErrors("name");
		return result.name;
	};

	const submit = (e: FormEvent) => {
		e.preventDefault();
		const name = checkName();
		if (name === null) return;
		form.transform((data) => ({ ...data, name }));
		form.post("/register", { onFinish: () => form.reset("password", "password_confirmation") });
	};

	return (
		<section className={card}>
			<Head title={t("account.register.title")} />
			<h1 className={heading}>{t("account.register.title")}</h1>
			<form onSubmit={submit} className="space-y-4">
				<Field
					label={t("account.common.name")}
					hint={t("account.common.name_hint")}
					autoComplete="name"
					autoCapitalize="words"
					minLength={NAME_MIN}
					maxLength={NAME_MAX}
					required
					value={form.data.name}
					onChange={(e) => {
						form.setData("name", capitalizeName(e.target.value));
						// Cleared while typing, not on blur, so the form does not
						// shift under a click on the submit button.
						if (form.errors.name) form.clearErrors("name");
					}}
					onBlur={() => form.data.name && checkName()}
					error={form.errors.name}
				/>
				<Field
					label={t("account.common.email")}
					type="email"
					autoComplete="email"
					minLength={EMAIL_MIN}
					maxLength={EMAIL_MAX}
					required
					value={form.data.email}
					onChange={(e) => form.setData("email", e.target.value)}
					error={form.errors.email}
				/>
				<PasswordField
					label={t("account.register.password")}
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
					{t("account.register.submit")}
				</button>
			</form>
			<SocialButtons providers={socialProviders} />
			<p className="mt-6 text-sm">
				<Link href="/login" className={link}>
					{t("account.register.have_account")}
				</Link>
			</p>
		</section>
	);
}
