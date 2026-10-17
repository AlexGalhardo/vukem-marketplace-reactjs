import { Head, Link, usePage } from "@inertiajs/react";
import type { JSX } from "react";

import { button, card, heading } from "../../components/ui";
import { useT } from "../../lib/i18n";
import type { SharedProps } from "../../types";

export default function VerifyEmail(): JSX.Element {
	const { t } = useT();
	const { auth } = usePage<SharedProps>().props;

	return (
		<section className={card}>
			<Head title={t("account.verify.title")} />
			<h1 className={heading}>{t("account.verify.title")}</h1>
			<p className="mb-4">{t("account.verify.body", { email: auth?.user?.email ?? "" })}</p>
			<Link href="/email/verification-notification" method="post" as="button" className={button}>
				{t("account.verify.resend")}
			</Link>
		</section>
	);
}
