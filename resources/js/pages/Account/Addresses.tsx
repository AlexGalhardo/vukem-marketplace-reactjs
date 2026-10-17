import { Head, router, useForm } from "@inertiajs/react";
import { type FormEvent, type JSX, useState } from "react";

import Field from "../../components/Field";
import { useT } from "../../lib/i18n";
import { serverMessage } from "../../lib/messages";
import type { Address } from "../../types";

interface Props {
	addresses: Address[];
	max: number;
}

const empty = {
	recipient: "",
	cep: "",
	street: "",
	number: "",
	complement: "",
	district: "",
	city: "",
	uf: "",
	phone: "",
	isDefault: false,
};

type FormData = typeof empty;

function AddressForm({ editing, onDone }: { editing: Address | null; onDone: () => void }): JSX.Element {
	const { t, locale } = useT();
	const form = useForm<FormData>(editing ? { ...empty, ...editing } : empty);
	const [lookupFailed, setLookupFailed] = useState(false);
	const err = (field: keyof FormData) => serverMessage(locale, form.errors[field]);

	// Autofill from ViaCEP through the server (GET /cep/{cep}).
	const lookup = async () => {
		const cep = form.data.cep.replace(/\D/g, "");
		setLookupFailed(false);
		if (cep.length !== 8) {
			setLookupFailed(true);
			return;
		}
		try {
			const res = await fetch(`/cep/${cep}`, { headers: { Accept: "application/json" } });
			if (!res.ok) throw new Error(String(res.status));
			const a = (await res.json()) as { street: string; district: string; city: string; uf: string };
			form.setData((d) => ({ ...d, street: a.street, district: a.district, city: a.city, uf: a.uf }));
		} catch {
			setLookupFailed(true);
		}
	};

	const submit = (e: FormEvent) => {
		e.preventDefault();
		const options = { preserveScroll: true, onSuccess: onDone };
		if (editing) form.put(`/account/addresses/${editing.id}`, options);
		else form.post("/account/addresses", options);
	};

	return (
		<form onSubmit={submit} className="grid gap-4 rounded-md bg-card p-5 shadow-card sm:grid-cols-6">
			<div className="sm:col-span-6">
				<Field
					label={t("checkout.addresses.recipient")}
					autoComplete="name"
					required
					value={form.data.recipient}
					onChange={(e) => form.setData("recipient", e.target.value)}
					error={err("recipient")}
				/>
			</div>
			<div className="flex items-end gap-2 sm:col-span-3">
				<div className="flex-1">
					<Field
						label={t("checkout.addresses.cep")}
						inputMode="numeric"
						autoComplete="postal-code"
						required
						maxLength={9}
						value={form.data.cep}
						onChange={(e) => form.setData("cep", e.target.value)}
						onBlur={lookup}
						error={err("cep")}
					/>
				</div>
				<button
					type="button"
					onClick={lookup}
					className="mb-px h-10 rounded-md border border-action-500 px-3 text-sm text-action-600 hover:bg-action-500/10"
				>
					{t("checkout.addresses.lookup")}
				</button>
			</div>
			{lookupFailed && (
				<p role="status" className="text-sm text-muted sm:col-span-6">
					{t("checkout.addresses.lookup_failed")}
				</p>
			)}
			<div className="sm:col-span-4">
				<Field
					label={t("checkout.addresses.street")}
					autoComplete="address-line1"
					required
					value={form.data.street}
					onChange={(e) => form.setData("street", e.target.value)}
					error={err("street")}
				/>
			</div>
			<div className="sm:col-span-2">
				<Field
					label={t("checkout.addresses.number")}
					required
					value={form.data.number}
					onChange={(e) => form.setData("number", e.target.value)}
					error={err("number")}
				/>
			</div>
			<div className="sm:col-span-3">
				<Field
					label={t("checkout.addresses.complement")}
					autoComplete="address-line2"
					value={form.data.complement}
					onChange={(e) => form.setData("complement", e.target.value)}
					error={err("complement")}
				/>
			</div>
			<div className="sm:col-span-3">
				<Field
					label={t("checkout.addresses.district")}
					required
					value={form.data.district}
					onChange={(e) => form.setData("district", e.target.value)}
					error={err("district")}
				/>
			</div>
			<div className="sm:col-span-4">
				<Field
					label={t("checkout.addresses.city")}
					autoComplete="address-level2"
					required
					value={form.data.city}
					onChange={(e) => form.setData("city", e.target.value)}
					error={err("city")}
				/>
			</div>
			<div className="sm:col-span-2">
				<Field
					label={t("checkout.addresses.uf")}
					autoComplete="address-level1"
					required
					maxLength={2}
					value={form.data.uf}
					onChange={(e) => form.setData("uf", e.target.value.toUpperCase())}
					error={err("uf")}
				/>
			</div>
			<div className="sm:col-span-3">
				<Field
					label={t("checkout.addresses.phone")}
					type="tel"
					autoComplete="tel"
					value={form.data.phone}
					onChange={(e) => form.setData("phone", e.target.value)}
					error={err("phone")}
				/>
			</div>
			<label className="flex items-center gap-2 text-sm sm:col-span-6">
				<input
					type="checkbox"
					checked={form.data.isDefault}
					onChange={(e) => form.setData("isDefault", e.target.checked)}
				/>
				{t("checkout.addresses.is_default")}
			</label>
			<div className="flex gap-3 sm:col-span-6">
				<button
					type="submit"
					disabled={form.processing}
					className="rounded-md bg-action-500 px-4 py-2 font-semibold text-on-action hover:bg-action-600 disabled:opacity-50"
				>
					{t("checkout.addresses.save")}
				</button>
				<button type="button" onClick={onDone} className="px-4 py-2 text-action-600 hover:underline">
					{t("checkout.addresses.cancel")}
				</button>
			</div>
		</form>
	);
}

export default function Addresses({ addresses, max }: Props): JSX.Element {
	const { t } = useT();
	const [editing, setEditing] = useState<Address | "new" | null>(addresses.length === 0 ? "new" : null);

	return (
		<>
			<Head title={t("checkout.addresses.title")} />
			<div className="mb-4 flex flex-wrap items-center justify-between gap-3">
				<h1 className="text-2xl font-semibold">{t("checkout.addresses.title")}</h1>
				{editing === null && addresses.length < max && (
					<button
						type="button"
						onClick={() => setEditing("new")}
						className="rounded-md bg-action-500 px-4 py-2 font-semibold text-on-action hover:bg-action-600"
					>
						{t("checkout.addresses.add")}
					</button>
				)}
			</div>
			{editing !== null && (
				<div className="mb-6">
					<AddressForm
						key={editing === "new" ? "new" : editing.id}
						editing={editing === "new" ? null : editing}
						onDone={() => setEditing(null)}
					/>
				</div>
			)}
			{addresses.length === 0 && editing === null && (
				<p className="text-muted">{t("checkout.addresses.empty")}</p>
			)}
			<ul className="grid gap-3 sm:grid-cols-2">
				{addresses.map((a) => (
					<li
						key={a.id}
						className={`rounded-md bg-card p-4 shadow-card ${a.isDefault ? "ring-2 ring-action-500" : "border border-line"}`}
					>
						<p className="font-medium">
							{a.recipient}
							{a.isDefault && (
								<span className="ml-2 rounded bg-action-500/10 px-2 py-0.5 text-xs text-action-600">
									{t("checkout.addresses.default")}
								</span>
							)}
						</p>
						<address className="mt-1 text-sm text-muted not-italic">
							{a.street}, {a.number}
							{a.complement && ` – ${a.complement}`}
							<br />
							{a.district}, {a.city}/{a.uf}
							<br />
							{a.cep.replace(/^(\d{5})(\d{3})$/, "$1-$2")}
						</address>
						<div className="mt-3 flex gap-4 text-sm">
							<button
								type="button"
								onClick={() => setEditing(a)}
								className="text-action-600 hover:underline"
							>
								{t("checkout.addresses.edit")}
							</button>
							{!a.isDefault && (
								<button
									type="button"
									onClick={() =>
										router.post(`/account/addresses/${a.id}/default`, {}, { preserveScroll: true })
									}
									className="text-action-600 hover:underline"
								>
									{t("checkout.addresses.make_default")}
								</button>
							)}
							<button
								type="button"
								onClick={() => router.delete(`/account/addresses/${a.id}`, { preserveScroll: true })}
								className="text-danger-600 hover:underline"
							>
								{t("checkout.addresses.delete")}
							</button>
						</div>
					</li>
				))}
			</ul>
		</>
	);
}
