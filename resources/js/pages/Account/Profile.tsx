import { Head, Link, useForm, usePage } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import { button, card, heading, link, muted } from "../../components/ui";
import { useT } from "../../lib/i18n";
import type { SharedProps } from "../../types";

export default function Profile(): JSX.Element {
	const { t } = useT();
	const user = usePage<SharedProps>().props.auth?.user;
	const form = useForm({ name: user?.name ?? "", email: user?.email ?? "", password: "" });
	const emailChanged = form.data.email.trim().toLowerCase() !== (user?.email ?? "");

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.put("/account", { preserveScroll: true, onFinish: () => form.reset("password") });
	};

	return (
		<section className={card}>
			<Head title={t("account.profile.title")} />
			<h1 className={heading}>{t("account.profile.heading")}</h1>
			{user && !user.email_verified_at && (
				<p className="mb-4 rounded-md bg-brand-100 px-3 py-2 text-sm text-brand-900">
					{t("account.profile.unverified")}{" "}
					<Link href="/email/verification-notification" method="post" as="button" className={link}>
						{t("account.profile.resend")}
					</Link>
				</p>
			)}
			<form onSubmit={submit} className="space-y-4">
				<Field
					label={t("account.common.name")}
					autoComplete="name"
					required
					value={form.data.name}
					onChange={(e) => form.setData("name", e.target.value)}
					error={form.errors.name}
				/>
				<Field
					label={t("account.common.email")}
					type="email"
					autoComplete="email"
					required
					value={form.data.email}
					onChange={(e) => form.setData("email", e.target.value)}
					error={form.errors.email}
				/>
				{(emailChanged || form.errors.password) && (
					<div>
						<Field
							label={t("account.common.current_password")}
							type="password"
							autoComplete="current-password"
							required
							value={form.data.password}
							onChange={(e) => form.setData("password", e.target.value)}
							error={form.errors.password}
						/>
						<p className={`mt-1 ${muted}`}>{t("account.profile.password_hint")}</p>
					</div>
				)}
				<button type="submit" disabled={form.processing} className={button}>
					{t("account.common.save")}
				</button>
			</form>
			<p className="mt-6 text-sm">
				<Link href="/account/security" className={link}>
					{t("account.profile.security")}
				</Link>
			</p>
		</section>
	);
}
