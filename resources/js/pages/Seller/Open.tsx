import { Head, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import ShopFields, { type ShopForm } from "../../components/catalog/ShopFields";
import { useT } from "../../lib/i18n";

export default function Open({ ufs }: { ufs: string[] }): JSX.Element {
	const { t } = useT();
	const form = useForm<ShopForm>({ name: "", description: "", ship_from_uf: "SP" });

	const submit = (e: FormEvent): void => {
		e.preventDefault();
		form.post("/sell");
	};

	return (
		<>
			<Head title={t("seller.open.title")} />
			<div className="mx-auto max-w-lg rounded-md bg-card p-6 shadow-sm">
				<h1 className="text-2xl font-semibold text-ink">{t("seller.open.title")}</h1>
				<p className="mt-1 text-sm text-muted">{t("seller.open.body")}</p>
				<form onSubmit={submit} className="mt-6 space-y-4">
					<ShopFields data={form.data} errors={form.errors} ufs={ufs} set={(k, v) => form.setData(k, v)} />
					<button
						type="submit"
						disabled={form.processing}
						className="w-full rounded-md bg-action-600 px-4 py-2.5 font-medium text-white hover:bg-action-700 disabled:opacity-50"
					>
						{t("seller.open.submit")}
					</button>
				</form>
			</div>
		</>
	);
}
