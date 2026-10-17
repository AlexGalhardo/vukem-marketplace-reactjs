import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import Field from "../../components/Field";
import { button, card, dangerButton, heading, link, secondaryButton, subheading } from "../../components/ui";
import { useT } from "../../lib/i18n";

interface Props {
	enabled: boolean;
	pending: boolean;
	secret: string | null;
	qr: string | null;
	recoveryCodes: string[] | null;
	recoveryCodesLeft: number;
}

export default function TwoFactor({
	enabled,
	pending,
	secret,
	qr,
	recoveryCodes,
	recoveryCodesLeft,
}: Props): JSX.Element {
	const { t } = useT();
	const confirm = useForm({ code: "" });
	const disable = useForm({ password: "" });

	const submitConfirm = (e: FormEvent) => {
		e.preventDefault();
		confirm.post("/user/two-factor/confirm", { onFinish: () => confirm.reset("code") });
	};

	const submitDisable = (e: FormEvent) => {
		e.preventDefault();
		disable.post("/user/two-factor/disable", { onFinish: () => disable.reset("password") });
	};

	return (
		<section className={card}>
			<Head title={t("account.two_factor.title")} />
			<h1 className={heading}>{t("account.two_factor.title")}</h1>
			{enabled ? (
				<div className="space-y-4">
					<p>{t("account.two_factor.on")}</p>
					{recoveryCodes ? (
						<>
							<p>
								<strong>{t("account.two_factor.store_codes")}</strong>{" "}
								{t("account.two_factor.store_codes_body")}
							</p>
							<ul className="grid grid-cols-2 gap-2 font-mono">
								{recoveryCodes.map((code) => (
									<li key={code}>{code}</li>
								))}
							</ul>
						</>
					) : (
						<p>{t("account.two_factor.codes_left", { count: recoveryCodesLeft })}</p>
					)}
					<Link href="/user/two-factor/recovery-codes" method="post" as="button" className={secondaryButton}>
						{t("account.two_factor.regenerate")}
					</Link>
					<h2 className={`pt-4 ${subheading}`}>{t("account.two_factor.turn_off")}</h2>
					<form onSubmit={submitDisable} className="space-y-4">
						<Field
							label={t("account.common.current_password")}
							type="password"
							autoComplete="current-password"
							required
							value={disable.data.password}
							onChange={(e) => disable.setData("password", e.target.value)}
							error={disable.errors.password}
						/>
						<button type="submit" disabled={disable.processing} className={dangerButton}>
							{t("account.two_factor.disable")}
						</button>
					</form>
				</div>
			) : pending && qr ? (
				<div className="space-y-4">
					<p>{t("account.two_factor.scan")}</p>
					<img
						src={qr}
						alt={t("account.two_factor.qr_alt")}
						width={200}
						height={200}
						className="rounded bg-white p-2"
					/>
					<p>
						{t("account.two_factor.setup_key")} <code className="font-mono">{secret}</code>
					</p>
					<form onSubmit={submitConfirm} className="space-y-4">
						<Field
							label={t("account.two_factor.code")}
							inputMode="numeric"
							autoComplete="one-time-code"
							required
							value={confirm.data.code}
							onChange={(e) => confirm.setData("code", e.target.value)}
							error={confirm.errors.code}
						/>
						<button type="submit" disabled={confirm.processing} className={button}>
							{t("account.two_factor.confirm")}
						</button>
					</form>
				</div>
			) : (
				<div className="space-y-4">
					<p>{t("account.two_factor.intro")}</p>
					<Link href="/user/two-factor" method="post" as="button" className={button}>
						{t("account.two_factor.enable")}
					</Link>
				</div>
			)}
			<p className="mt-6 text-sm">
				<Link href="/account/security" className={link}>
					{t("account.security.title")}
				</Link>
			</p>
		</section>
	);
}
