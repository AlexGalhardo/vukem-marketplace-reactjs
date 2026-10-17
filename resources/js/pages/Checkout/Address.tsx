import { Head, Link, useForm } from "@inertiajs/react";
import type { FormEvent, JSX } from "react";

import CheckoutSteps from "../../components/commerce/CheckoutSteps";
import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import type { Address } from "../../types";

interface Props {
	addresses: Address[];
	selectedId: number;
}

export default function CheckoutAddress({ addresses, selectedId }: Props): JSX.Element {
	const { t, locale } = useT();
	const initial = selectedId || addresses.find((a) => a.isDefault)?.id || addresses[0]?.id || 0;
	const form = useForm({ addressId: initial });

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/checkout/address");
	};

	return (
		<>
			<Head title={t("checkout.address.title")} />
			<CheckoutSteps current="address" />
			<h1 className="mb-4 text-2xl font-semibold">{t("checkout.address.title")}</h1>
			{addresses.length === 0 ? (
				<div className="rounded-md bg-card p-6">
					<p className="mb-3 text-muted">{t("checkout.address.none")}</p>
					<Link href="/account/addresses" className="font-medium text-action-600 hover:underline">
						{t("checkout.address.add")}
					</Link>
				</div>
			) : (
				<form onSubmit={submit} className="max-w-2xl">
					<fieldset className="flex flex-col gap-3">
						<legend className="sr-only">{t("checkout.address.title")}</legend>
						{addresses.map((a) => (
							<label
								key={a.id}
								className="flex cursor-pointer gap-3 rounded-md border border-line bg-card p-4 has-checked:border-action-500 has-checked:ring-1 has-checked:ring-action-500"
							>
								<input
									type="radio"
									name="addressId"
									value={a.id}
									checked={form.data.addressId === a.id}
									onChange={() => form.setData("addressId", a.id)}
									className="mt-1"
								/>
								<span>
									<span className="block font-medium">{a.recipient}</span>
									<span className="block text-sm text-muted">
										{a.street}, {a.number}
										{a.complement && ` – ${a.complement}`}, {a.district}, {a.city}/{a.uf}
									</span>
								</span>
							</label>
						))}
					</fieldset>
					{form.errors.addressId && (
						<p role="alert" className="mt-2 text-sm text-danger-600">
							{serverMessage(locale, form.errors.addressId)}
						</p>
					)}
					<div className="mt-4 flex flex-wrap items-center gap-4">
						<button
							type="submit"
							disabled={form.processing || !form.data.addressId}
							className="rounded-md bg-action-500 px-6 py-3 font-semibold text-white hover:bg-action-600 disabled:opacity-50"
						>
							{t("checkout.address.choose")}
						</button>
						<Link href="/account/addresses" className="text-sm text-action-600 hover:underline">
							{t("checkout.address.manage")}
						</Link>
					</div>
				</form>
			)}
		</>
	);
}
