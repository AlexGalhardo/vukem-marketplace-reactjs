import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import SocialButtons from "../../components/SocialButtons";
import { button, card, heading, link } from "../../components/ui";
import { useT } from "../../lib/i18n";
import type { AuthPageProps } from "../../types";

export default function Login({ socialProviders = [] }: AuthPageProps): JSX.Element {
	const { t } = useT();
	const form = useForm({ email: "", password: "", remember: false });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/login", { onFinish: () => form.reset("password") });
	};

	return (
		<section className={card}>
			<Head title={t("account.login.title")} />
			<h1 className={heading}>{t("account.login.title")}</h1>
			<form onSubmit={submit} className="space-y-4">
				<Field
					label={t("account.common.email")}
					type="email"
					autoComplete="username"
					required
					value={form.data.email}
					onChange={(e) => form.setData("email", e.target.value)}
					error={form.errors.email}
				/>
				<Field
					label={t("account.common.password")}
					type="password"
					autoComplete="current-password"
					required
					value={form.data.password}
					onChange={(e) => form.setData("password", e.target.value)}
					error={form.errors.password}
				/>
				<label className="flex items-center gap-2 text-sm">
					<input
						type="checkbox"
						checked={form.data.remember}
						onChange={(e) => form.setData("remember", e.target.checked)}
					/>
					{t("account.login.remember")}
				</label>
				<button type="submit" disabled={form.processing} className={button}>
					{t("account.login.submit")}
				</button>
			</form>
			<SocialButtons providers={socialProviders} />
			<p className="mt-6 flex flex-wrap gap-4 text-sm">
				<Link href="/forgot-password" className={link}>
					{t("account.login.forgot")}
				</Link>
				<Link href="/register" className={link}>
					{t("account.login.register")}
				</Link>
			</p>
		</section>
	);
}
