import { Head } from "@inertiajs/react";
import type { JSX } from "react";

import { type MessageKey, useT } from "../lib/i18n";

const benefits: MessageKey[] = ["home.benefits.pix", "home.benefits.shipping", "home.benefits.protection"];

export default function Home(): JSX.Element {
	const { t } = useT();

	return (
		<>
			<Head title={t("app.name")} />
			<section className="rounded-lg bg-card px-6 py-12 text-center shadow-sm">
				<h1 className="text-3xl font-semibold tracking-tight text-ink sm:text-4xl">{t("home.hero_title")}</h1>
				<p className="mx-auto mt-3 max-w-xl text-muted">{t("home.hero_body")}</p>
				<p className="mt-6 inline-block rounded-md bg-action-600 px-5 py-2.5 font-medium text-white">
					{t("home.coming_soon")}
				</p>
			</section>
			<ul className="mt-6 grid gap-4 sm:grid-cols-3">
				{benefits.map((key) => (
					<li key={key} className="rounded-lg bg-card px-4 py-5 text-sm font-medium text-ink shadow-sm">
						{t(key)}
					</li>
				))}
			</ul>
		</>
	);
}
