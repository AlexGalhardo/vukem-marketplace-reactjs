import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import { button, heading, link, muted, panel, secondaryButton, subheading } from "../../components/ui";
import { useT } from "../../lib/i18n";
import type { SecurityProps } from "../../types";

export default function Security({ twoFactorEnabled, recoveryCodesLeft }: SecurityProps): JSX.Element {
	const { t } = useT();
	const password = useForm({ current_password: "", password: "", password_confirmation: "" });
	const sessions = useForm({ password: "" });

	const changePassword = (e: FormEvent) => {
		e.preventDefault();
		password.put("/account/password", { preserveScroll: true, onFinish: () => password.reset() });
	};

	const logoutOthers = (e: FormEvent) => {
		e.preventDefault();
		sessions.post("/account/sessions/logout-others", { preserveScroll: true, onFinish: () => sessions.reset() });
	};

	return (
		<div className="mx-auto max-w-2xl space-y-6">
			<Head title={t("account.security.title")} />
			<h1 className={heading}>{t("account.security.title")}</h1>

			<section className={panel} aria-labelledby="password-title">
				<h2 id="password-title" className={subheading}>
					{t("account.security.password_title")}
				</h2>
				<form onSubmit={changePassword} className="space-y-4">
					<Field
						label={t("account.common.current_password")}
						type="password"
						autoComplete="current-password"
						required
						value={password.data.current_password}
						onChange={(e) => password.setData("current_password", e.target.value)}
						error={password.errors.current_password}
					/>
					<Field
						label={t("account.common.new_password")}
						type="password"
						autoComplete="new-password"
						minLength={12}
						required
						value={password.data.password}
						onChange={(e) => password.setData("password", e.target.value)}
						error={password.errors.password}
					/>
					<Field
						label={t("account.common.confirm_password")}
						type="password"
						autoComplete="new-password"
						required
						value={password.data.password_confirmation}
						onChange={(e) => password.setData("password_confirmation", e.target.value)}
						error={password.errors.password_confirmation}
					/>
					<button type="submit" disabled={password.processing} className={button}>
						{t("account.security.password_submit")}
					</button>
				</form>
			</section>

			<section className={panel} aria-labelledby="two-factor-title">
				<h2 id="two-factor-title" className={subheading}>
					{t("account.security.two_factor_title")}
				</h2>
				<p className="mb-4">
					{twoFactorEnabled
						? t("account.security.two_factor_on", { count: recoveryCodesLeft })
						: t("account.security.two_factor_off")}
				</p>
				<Link href="/user/two-factor" className={secondaryButton}>
					{t("account.security.two_factor_manage")}
				</Link>
			</section>

			<section className={panel} aria-labelledby="sessions-title">
				<h2 id="sessions-title" className={subheading}>
					{t("account.security.sessions_title")}
				</h2>
				<p className={`mb-4 ${muted}`}>{t("account.security.sessions_body")}</p>
				<form onSubmit={logoutOthers} className="space-y-4">
					<Field
						label={t("account.common.current_password")}
						type="password"
						autoComplete="current-password"
						required
						value={sessions.data.password}
						onChange={(e) => sessions.setData("password", e.target.value)}
						error={sessions.errors.password}
					/>
					<button type="submit" disabled={sessions.processing} className={secondaryButton}>
						{t("account.security.sessions_submit")}
					</button>
				</form>
			</section>

			<p className="text-sm">
				<Link href="/account" className={link}>
					{t("account.security.back_profile")}
				</Link>
			</p>
		</div>
	);
}
