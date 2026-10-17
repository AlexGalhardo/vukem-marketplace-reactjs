import { Head, Link, router, useForm, usePage } from "@inertiajs/react";
import { type ChangeEvent, type FormEvent, type JSX, useState } from "react";
import ProductImage from "../../components/catalog/ProductImage";
import SellerNav from "../../components/catalog/SellerNav";
import StatusBadge from "../../components/catalog/StatusBadge";
import Field from "../../components/Field";
import { useT } from "../../lib/i18n";
import { centsToInput, toCents } from "../../lib/money";
import type { CategoryRef, ImageView, SellerProduct, SharedProps } from "../../types";

interface Props {
	product: SellerProduct | null;
	categories: CategoryRef[];
	maxImages: number;
}

interface FormData {
	title: string;
	description: string;
	category: string;
	price: string;
	stock: string;
	weight_grams: string;
}

const controlClass =
	"w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus:border-action-500 focus:outline-none";

export default function ProductForm({ product, categories, maxImages }: Props): JSX.Element {
	const { t } = useT();
	const form = useForm<FormData>({
		title: product?.title ?? "",
		description: product?.description ?? "",
		category: product?.category ?? "",
		price: product ? centsToInput(product.price_cents) : "",
		stock: product ? String(product.stock) : "1",
		weight_grams: product ? String(product.weight_grams) : "",
	});

	// The browser converts the typed amount to integer cents; the server
	// validates it again (price_cents errors come back under that key).
	form.transform((data) => ({
		title: data.title,
		description: data.description,
		category: data.category,
		price_cents: toCents(data.price),
		stock: Number(data.stock),
		weight_grams: Number(data.weight_grams),
	}));

	const submit = (e: FormEvent): void => {
		e.preventDefault();
		if (product) form.put(`/seller/products/${product.id}`, { preserveScroll: true });
		else form.post("/seller/products");
	};

	const title = product ? t("seller.form.edit_title") : t("seller.form.new_title");
	const errors = form.errors as Partial<Record<string, string>>;

	return (
		<>
			<Head title={title} />
			<SellerNav current="/seller/products" />
			<div className="mb-4 flex flex-wrap items-center gap-3">
				<h1 className="text-2xl font-semibold text-ink">{title}</h1>
				{product && <StatusBadge status={product.status} />}
				{product?.status === "published" && (
					<Link href={`/p/${product.slug}`} className="ml-auto text-sm text-action-600 hover:underline">
						{t("seller.form.view_public")}
					</Link>
				)}
			</div>
			<div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
				<form onSubmit={submit} className="space-y-4 rounded-md bg-card p-5 shadow-card">
					<Field
						label={t("seller.form.title")}
						required
						maxLength={120}
						value={form.data.title}
						onChange={(e) => form.setData("title", e.target.value)}
						error={errors.title}
					/>
					<div className="flex flex-col gap-1">
						<label htmlFor="category" className="text-sm font-medium text-ink">
							{t("seller.form.category")}
						</label>
						<select
							id="category"
							value={form.data.category}
							onChange={(e) => form.setData("category", e.target.value)}
							className={`${controlClass} h-10`}
						>
							<option value="">{t("seller.form.no_category")}</option>
							{categories.map((c) => (
								<option key={c.slug} value={c.slug}>
									{c.name}
								</option>
							))}
						</select>
						{errors.category && <p className="text-sm text-danger-600">{errors.category}</p>}
					</div>
					<div className="grid gap-4 sm:grid-cols-3">
						<Field
							label={t("seller.form.price")}
							required
							inputMode="decimal"
							placeholder="0,00"
							value={form.data.price}
							onChange={(e) => form.setData("price", e.target.value)}
							error={errors.price_cents}
						/>
						<Field
							label={t("seller.form.stock")}
							required
							type="number"
							min={0}
							step={1}
							value={form.data.stock}
							onChange={(e) => form.setData("stock", e.target.value)}
							error={errors.stock}
						/>
						<Field
							label={t("seller.form.weight")}
							required
							type="number"
							min={1}
							step={1}
							value={form.data.weight_grams}
							onChange={(e) => form.setData("weight_grams", e.target.value)}
							error={errors.weight_grams}
						/>
					</div>
					<div className="flex flex-col gap-1">
						<label htmlFor="description" className="text-sm font-medium text-ink">
							{t("seller.form.description")}
						</label>
						<textarea
							id="description"
							rows={8}
							maxLength={5000}
							value={form.data.description}
							onChange={(e) => form.setData("description", e.target.value)}
							className={controlClass}
						/>
						{errors.description && <p className="text-sm text-danger-600">{errors.description}</p>}
					</div>
					<button
						type="submit"
						disabled={form.processing}
						className="rounded-md bg-action-500 px-4 py-2 font-medium text-on-action hover:bg-action-600 disabled:opacity-50"
					>
						{product ? t("seller.form.save") : t("seller.form.create")}
					</button>
					{!product && <p className="text-sm text-muted">{t("seller.form.draft_hint")}</p>}
				</form>

				{product && (
					<div className="space-y-6">
						<StatusActions product={product} />
						<Images product={product} maxImages={maxImages} />
					</div>
				)}
			</div>
		</>
	);
}

function StatusActions({ product }: { product: SellerProduct }): JSX.Element {
	const { t } = useT();
	const set = (status: string): void => {
		router.post(`/seller/products/${product.id}/status`, { status }, { preserveScroll: true });
	};
	const button = "w-full rounded-md px-3 py-2 text-sm font-medium";

	if (product.status === "suspended") {
		return (
			<p role="alert" className="rounded-md bg-danger-600/10 p-4 text-sm text-danger-600">
				{t("seller.form.suspended")}
			</p>
		);
	}

	return (
		<section aria-labelledby="visibility-title" className="space-y-2 rounded-md bg-card p-4 shadow-card">
			<h2 id="visibility-title" className="font-medium text-ink">
				{t("seller.form.visibility")}
			</h2>
			{product.status !== "published" && (
				<button
					type="button"
					onClick={() => set("published")}
					className={`${button} bg-success-600 text-on-status hover:opacity-90`}
				>
					{t("seller.form.publish")}
				</button>
			)}
			{product.status === "published" && (
				<button
					type="button"
					onClick={() => set("draft")}
					className={`${button} border border-line text-ink hover:bg-line`}
				>
					{t("seller.form.unpublish")}
				</button>
			)}
			{product.status !== "archived" && (
				<button type="button" onClick={() => set("archived")} className={`${button} text-muted hover:text-ink`}>
					{t("seller.form.archive")}
				</button>
			)}
		</section>
	);
}

function Images({ product, maxImages }: { product: SellerProduct; maxImages: number }): JSX.Element {
	const { t } = useT();
	const errors = usePage<SharedProps>().props.errors;
	const [uploading, setUploading] = useState(false);
	const base = `/seller/products/${product.id}/images`;
	const free = maxImages - product.images.length;
	const uploadError = Object.entries(errors).find(
		([k]) => k === "images" || k.startsWith("images[") || k.startsWith("images."),
	)?.[1];

	const upload = (e: ChangeEvent<HTMLInputElement>): void => {
		const files = Array.from(e.target.files ?? []);
		e.target.value = "";
		if (files.length === 0) return;
		router.post(
			base,
			{ images: files },
			{
				forceFormData: true,
				preserveScroll: true,
				onStart: () => setUploading(true),
				onFinish: () => setUploading(false),
			},
		);
	};

	const move = (from: number, to: number): void => {
		const ids = product.images.map((img) => img.id);
		const [id] = ids.splice(from, 1);
		if (id === undefined) return;
		ids.splice(to, 0, id);
		router.put(`${base}/order`, { ids }, { preserveScroll: true });
	};

	const remove = (img: ImageView): void => {
		router.delete(`${base}/${img.id}`, { preserveScroll: true });
	};

	return (
		<section aria-labelledby="images-title" className="rounded-md bg-card p-4 shadow-card">
			<h2 id="images-title" className="font-medium text-ink">
				{t("seller.images.title", { count: product.images.length, max: maxImages })}
			</h2>
			{product.images.length > 0 && (
				<ol className="mt-3 grid grid-cols-2 gap-3">
					{product.images.map((img, i) => (
						<li key={img.id} className="rounded-md border border-line p-1">
							<ProductImage src={img.url_320} alt={t("seller.images.photo", { n: i + 1 })} />
							{img.processing && (
								<p className="pt-1 text-center text-xs text-muted">{t("seller.images.processing")}</p>
							)}
							<div className="flex justify-between gap-1 pt-1">
								<button
									type="button"
									disabled={i === 0}
									onClick={() => move(i, i - 1)}
									aria-label={t("seller.images.move_left", { n: i + 1 })}
									className="rounded px-2 text-muted hover:text-ink disabled:opacity-30"
								>
									<span aria-hidden="true">‹</span>
								</button>
								<button
									type="button"
									onClick={() => remove(img)}
									aria-label={t("seller.images.remove", { n: i + 1 })}
									className="rounded px-2 text-xs text-danger-600 hover:underline"
								>
									{t("seller.images.remove_short")}
								</button>
								<button
									type="button"
									disabled={i === product.images.length - 1}
									onClick={() => move(i, i + 1)}
									aria-label={t("seller.images.move_right", { n: i + 1 })}
									className="rounded px-2 text-muted hover:text-ink disabled:opacity-30"
								>
									<span aria-hidden="true">›</span>
								</button>
							</div>
						</li>
					))}
				</ol>
			)}
			{free > 0 ? (
				<label className="mt-3 block cursor-pointer rounded-md border border-dashed border-action-600 px-3 py-4 text-center text-sm text-action-600 hover:bg-action-500/10 has-focus-visible:outline-2 has-focus-visible:outline-action-500">
					{uploading ? t("seller.images.uploading") : t("seller.images.add", { free })}
					<input
						type="file"
						name="images"
						multiple
						accept="image/jpeg,image/png,image/gif,image/webp"
						className="sr-only"
						onChange={upload}
						disabled={uploading}
					/>
				</label>
			) : (
				<p className="mt-3 text-sm text-muted">{t("seller.images.full")}</p>
			)}
			<p className="mt-2 text-xs text-muted">{t("seller.images.hint")}</p>
			{uploadError && (
				<p role="alert" className="mt-2 text-sm text-danger-600">
					{uploadError}
				</p>
			)}
		</section>
	);
}
