import { Head, Link } from "@inertiajs/react";
import { type JSX, useState } from "react";

import FreightBox from "../../components/catalog/FreightBox";
import Price from "../../components/catalog/Price";
import ProductGrid from "../../components/catalog/ProductGrid";
import ProductImage from "../../components/catalog/ProductImage";
import { useT } from "../../lib/i18n";
import type { ProductCard, ProductDetail } from "../../types";

interface Props {
	product: ProductDetail;
	more: ProductCard[];
}

export default function Product({ product, more }: Props): JSX.Element {
	const { t, locale } = useT();
	const [current, setCurrent] = useState(0);
	const image = product.images[current];
	const since = new Intl.DateTimeFormat(locale, { year: "numeric" }).format(new Date(product.shop.since));

	return (
		<>
			<Head title={product.title} />
			{product.category && (
				<nav aria-label={t("catalog.product.breadcrumb")} className="mb-3 text-sm">
					<Link href={`/c/${product.category.slug}`} className="text-action-600 hover:underline">
						{product.category.name}
					</Link>
				</nav>
			)}
			<article className="grid gap-6 rounded-md bg-card p-4 shadow-sm md:grid-cols-[1fr_20rem] md:p-6">
				<div className="grid gap-3 sm:grid-cols-[4rem_1fr]">
					{product.images.length > 1 && (
						<ul
							aria-label={t("catalog.product.photos")}
							className="order-last flex gap-2 sm:order-first sm:flex-col"
						>
							{product.images.map((img, i) => (
								<li key={img.id}>
									<button
										type="button"
										onClick={() => setCurrent(i)}
										onMouseEnter={() => setCurrent(i)}
										aria-label={t("catalog.product.show_photo", { n: i + 1 })}
										aria-pressed={i === current}
										className="block w-16 overflow-hidden rounded-md border border-line aria-pressed:border-action-600 aria-pressed:ring-1 aria-pressed:ring-action-600"
									>
										<ProductImage src={img.url_320} alt="" />
									</button>
								</li>
							))}
						</ul>
					)}
					<div className={product.images.length > 1 ? "" : "sm:col-span-2"}>
						{image ? (
							<img
								src={image.url_1280}
								srcSet={
									image.processing
										? undefined
										: `${image.url_320} 320w, ${image.url_640} 640w, ${image.url_1280} 1280w`
								}
								sizes="(min-width: 768px) 50vw, 100vw"
								alt={image.alt || product.title}
								className="mx-auto aspect-square w-full max-w-xl object-contain"
							/>
						) : (
							<ProductImage src="" alt="" className="mx-auto max-w-xl" />
						)}
					</div>
				</div>

				<div className="flex flex-col gap-4">
					<div>
						<h1 className="text-xl font-semibold leading-snug text-ink">{product.title}</h1>
						{product.rating_count > 0 && (
							<p className="mt-1 text-sm text-muted">
								{t("catalog.product.rating", {
									avg: (product.rating_avg_x100 / 100).toLocaleString(locale, {
										maximumFractionDigits: 1,
									}),
									count: product.rating_count,
								})}
							</p>
						)}
					</div>
					<Price cents={product.price_cents} size="lg" />
					<p className={`text-sm ${product.stock > 0 ? "text-success-600" : "text-danger-600"}`}>
						{product.stock > 0
							? t("catalog.product.in_stock", { count: product.stock })
							: t("catalog.product.out_of_stock")}
					</p>
					<FreightBox product={product} />
					<section aria-labelledby="seller-title" className="rounded-md border border-line p-3 text-sm">
						<h2 id="seller-title" className="text-muted">
							{t("catalog.product.sold_by")}
						</h2>
						<Link href={`/s/${product.shop.slug}`} className="font-medium text-action-600 hover:underline">
							{product.shop.name}
						</Link>
						<p className="text-muted">{t("catalog.product.seller_since", { year: since })}</p>
					</section>
				</div>

				<section aria-labelledby="description-title" className="md:col-span-2">
					<h2 id="description-title" className="mb-2 text-lg font-semibold text-ink">
						{t("catalog.product.description")}
					</h2>
					<p className="max-w-prose whitespace-pre-line text-muted">
						{product.description || t("catalog.product.no_description")}
					</p>
				</section>
			</article>

			{more.length > 0 && (
				<section aria-labelledby="more-title" className="mt-8">
					<h2 id="more-title" className="mb-3 text-xl font-semibold text-ink">
						{t("catalog.product.more_from", { shop: product.shop.name })}
					</h2>
					<ProductGrid products={more} />
				</section>
			)}
		</>
	);
}
