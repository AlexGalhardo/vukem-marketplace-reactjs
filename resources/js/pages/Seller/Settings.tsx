import { Head, router, useForm, usePage } from "@inertiajs/react";
import { type ChangeEvent, type FormEvent, type JSX, useState } from "react";

import SellerNav from "../../components/catalog/SellerNav";
import ShopFields, { type ShopForm } from "../../components/catalog/ShopFields";
import { useT } from "../../lib/i18n";
import type { SharedProps, ShopView } from "../../types";

export default function Settings({ shop, ufs }: { shop: ShopView; ufs: string[] }): JSX.Element {
	const { t } = useT();
	const errors = usePage<SharedProps>().props.errors;
	const [uploading, setUploading] = useState(false);
	const form = useForm<ShopForm>({ name: shop.name, description: shop.description, ship_from_uf: shop.ship_from_uf });
	const logoError = errors.logo ?? errors.images;

	const submit = (e: FormEvent): void => {
		e.preventDefault();
		form.put("/seller/settings", { preserveScroll: true });
	};

	const uploadLogo = (e: ChangeEvent<HTMLInputElement>): void => {
		const file = e.target.files?.[0];
		if (!file) return;
		router.post(
			"/seller/settings/logo",
			{ logo: file },
			{
				forceFormData: true,
				preserveScroll: true,
				onStart: () => setUploading(true),
				onFinish: () => setUploading(false),
			},
		);
	};

	return (
		<>
			<Head title={t("seller.settings.title")} />
			<SellerNav current="/seller/settings" shopSlug={shop.slug} />
			<div className="grid gap-6 md:grid-cols-[1fr_16rem]">
				<form onSubmit={submit} className="space-y-4 rounded-md bg-card p-5 shadow-card">
					<h1 className="text-xl font-semibold text-ink">{t("seller.settings.title")}</h1>
					<ShopFields data={form.data} errors={form.errors} ufs={ufs} set={(k, v) => form.setData(k, v)} />
					<button
						type="submit"
						disabled={form.processing}
						className="rounded-md bg-action-500 px-4 py-2 font-medium text-on-action hover:bg-action-600 disabled:opacity-50"
					>
						{t("seller.settings.save")}
					</button>
				</form>
				<section aria-labelledby="logo-title" className="rounded-md bg-card p-5 shadow-card">
					<h2 id="logo-title" className="font-medium text-ink">
						{t("seller.settings.logo")}
					</h2>
					<div className="mx-auto mt-3 size-28 overflow-hidden rounded-full border border-line bg-white">
						{shop.logo_url && <img src={shop.logo_url} alt="" className="size-full object-cover" />}
					</div>
					<label className="mt-4 block cursor-pointer rounded-md border border-action-600 px-3 py-2 text-center text-sm text-action-600 hover:bg-action-500/10 has-focus-visible:outline-2 has-focus-visible:outline-action-500">
						{uploading ? t("seller.images.uploading") : t("seller.settings.logo_upload")}
						<input
							type="file"
							accept="image/jpeg,image/png,image/gif,image/webp"
							className="sr-only"
							onChange={uploadLogo}
							disabled={uploading}
						/>
					</label>
					<p className="mt-2 text-xs text-muted">{t("seller.images.hint")}</p>
					{logoError && <p className="mt-2 text-sm text-danger-600">{logoError}</p>}
				</section>
			</div>
		</>
	);
}
