import { Head, Link, usePage } from "@inertiajs/react";
import type { JSX } from "react";

import { heading, link, panel } from "../components/ui";
import { useT } from "../lib/i18n";
import type { SharedProps } from "../types";

export default function Dashboard(): JSX.Element {
	const { t } = useT();
	const user = usePage<SharedProps>().props.auth?.user;

	return (
		<section className={panel}>
			<Head title={t("account.dashboard.title")} />
			<h1 className={heading}>{t("account.dashboard.hello", { name: user?.name ?? "" })}</h1>
			{user?.email_verified_at ? (
				<p>{t("account.dashboard.verified")}</p>
			) : (
				<p>
					<Link href="/email/verify" className={link}>
						{t("account.dashboard.verify")}
					</Link>
				</p>
			)}
			<ul className="mt-4 space-y-2">
				<li>
					<Link href="/account" className={link}>
						{t("account.dashboard.settings")}
					</Link>
				</li>
				<li>
					<Link href="/user/two-factor" className={link}>
						{t("account.two_factor.title")}
					</Link>
				</li>
			</ul>
		</section>
	);
}
