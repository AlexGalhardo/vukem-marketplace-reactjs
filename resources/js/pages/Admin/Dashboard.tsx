import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import { link } from "../../components/ui";
import { useT } from "../../lib/i18n";

export default function AdminDashboard(): JSX.Element {
	const { t } = useT();

	return (
		<>
			<Head title={t("admin.title")} />
			<h1 className="text-2xl font-semibold text-ink">{t("admin.title")}</h1>
			<p className="mt-2 text-muted">{t("admin.intro")}</p>
			<p className="mt-4">
				<Link href="/admin/flags" className={link}>
					{t("admin.flags_link")}
				</Link>
			</p>
			<p className="mt-2">
				<Link href="/admin/questions" className={link}>
					{t("qna.admin.link")}
				</Link>
			</p>
		</>
	);
}
