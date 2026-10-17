import { Head, useForm } from "@inertiajs/react";
import { type FormEvent, type JSX, useState } from "react";

import Field from "../../components/Field";
import { button, card, heading, link } from "../../components/ui";
import { useT } from "../../lib/i18n";

export default function TwoFactorChallenge(): JSX.Element {
	const { t } = useT();
	const [recovery, setRecovery] = useState(false);
	const form = useForm({ code: "", recovery_code: "" });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/two-factor-challenge", { onFinish: () => form.reset() });
	};

	return (
		<section className={card}>
			<Head title={t("account.challenge.title")} />
			<h1 className={heading}>{t("account.challenge.title")}</h1>
			<p className="mb-4">
				{recovery ? t("account.challenge.enter_recovery") : t("account.challenge.enter_code")}
			</p>
			<form onSubmit={submit} className="space-y-4">
				{recovery ? (
					<Field
						label={t("account.challenge.recovery_code")}
						autoComplete="off"
						required
						value={form.data.recovery_code}
						onChange={(e) => form.setData("recovery_code", e.target.value)}
						error={form.errors.recovery_code}
					/>
				) : (
					<Field
						label={t("account.two_factor.code")}
						inputMode="numeric"
						autoComplete="one-time-code"
						autoFocus
						required
						value={form.data.code}
						onChange={(e) => form.setData("code", e.target.value)}
						error={form.errors.code}
					/>
				)}
				<button type="submit" disabled={form.processing} className={button}>
					{t("account.challenge.submit")}
				</button>
			</form>
			<p className="mt-6 text-sm">
				<button
					type="button"
					onClick={() => {
						// Only one field is sent filled: the server checks the app code first.
						setRecovery(!recovery);
						form.reset();
						form.clearErrors();
					}}
					className={link}
				>
					{recovery ? t("account.challenge.use_app") : t("account.challenge.use_recovery")}
				</button>
			</p>
		</section>
	);
}
