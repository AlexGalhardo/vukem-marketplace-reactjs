import { Head, Link } from "@inertiajs/react";
import { type JSX, useState } from "react";

import Installments from "../../components/catalog/Installments";
import Price from "../../components/catalog/Price";
import ProductImage from "../../components/catalog/ProductImage";
import ProductQuestions from "../../components/catalog/ProductQuestions";
import Rating from "../../components/catalog/Rating";
import Shelf from "../../components/catalog/Shelf";
import ChatWithSeller from "../../components/chat/ChatWithSeller";
import AddToCartButton from "../../components/commerce/AddToCartButton";
import FreightBox from "../../components/commerce/FreightBox";
import { ChevronRightIcon, StoreIcon, TruckIcon } from "../../components/icons";
import ReviewsSection from "../../components/reviews/ReviewsSection";
import { useT } from "../../lib/i18n";
import type { Page, ProductCard, ProductDetail, PublicQuestion, QnaMeta, ReviewsBlock } from "../../types";

interface Props {
	product: ProductDetail;
	more: ProductCard[];
	questions: Page<PublicQuestion>;
	qna: QnaMeta;
	reviews?: ReviewsBlock;
}

export default function Product({ product, more, questions, qna, reviews }: Props): JSX.Element {
	const { t, locale } = useT();
	const [current, setCurrent] = useState(0);
	const image = product.images[current];
	const since = new Intl.DateTimeFormat(locale, { year: "numeric" }).format(new Date(product.shop.since));

	return (
		<>
			<Head title={product.title} />
			{product.category && (
				<nav aria-label={t("catalog.product.breadcrumb")} className="mb-3 text-sm">
					<ol className="flex items-center gap-1 text-muted">
						<li>
							<Link href="/search" className="text-action-600 hover:underline">
								{t("storefront.product.all")}
							</Link>
						</li>
						<li aria-hidden="true">
							<ChevronRightIcon className="size-3.5" />
						</li>
						<li>
							<Link href={`/c/${product.category.slug}`} className="text-action-600 hover:underline">
								{product.category.name}
							</Link>
						</li>
					</ol>
				</nav>
			)}

			<article className="grid gap-6 rounded-md bg-card p-4 shadow-card md:p-6 lg:grid-cols-[minmax(0,1fr)_19rem]">
				<div className="grid gap-5 md:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)] md:grid-rows-[auto_1fr] md:gap-x-8">
					<div className="md:col-start-2 md:row-start-1">
						<Rating avgX100={product.rating_avg_x100} count={product.rating_count} size="md" />
						<h1 className="mt-1 text-xl leading-snug font-semibold break-words text-ink sm:text-2xl">
							{product.title}
						</h1>
					</div>

					<div className="grid gap-3 sm:grid-cols-[3.5rem_1fr] md:col-start-1 md:row-span-2 md:row-start-1">
						{product.images.length > 1 && (
							<ul
								aria-label={t("catalog.product.photos")}
								className="relative order-last flex gap-2 overflow-x-auto scrollbar-none sm:order-first sm:flex-col"
							>
								{product.images.map((img, i) => (
									<li key={img.id} className="shrink-0">
										<button
											type="button"
											onClick={() => setCurrent(i)}
											onMouseEnter={() => setCurrent(i)}
											aria-label={t("catalog.product.show_photo", { n: i + 1 })}
											aria-pressed={i === current}
											className="block w-14 overflow-hidden rounded-md border border-line p-0.5 aria-pressed:border-2 aria-pressed:border-action-500"
										>
											<ProductImage src={img.url_320} alt="" />
										</button>
									</li>
								))}
							</ul>
						)}
						<div className={product.images.length > 1 ? "" : "sm:col-span-2"}>
							{image ? (
								<div className="flex aspect-square items-center justify-center overflow-hidden rounded-md bg-white">
									<img
										src={image.url_1280}
										srcSet={
											image.processing
												? undefined
												: `${image.url_320} 320w, ${image.url_640} 640w, ${image.url_1280} 1280w`
										}
										sizes="(min-width: 1024px) 34vw, (min-width: 768px) 50vw, 100vw"
										alt={image.alt || product.title}
										width={image.width || undefined}
										height={image.height || undefined}
										className="size-full object-contain"
									/>
								</div>
							) : (
								<ProductImage src="" alt="" className="rounded-md" />
							)}
						</div>
					</div>

					<div className="flex flex-col gap-3 md:col-start-2 md:row-start-2">
						<div>
							<Price cents={product.price_cents} size="lg" />
							<Installments cents={product.price_cents} className="mt-1 text-base" />
						</div>
						<p className="flex items-start gap-2 text-sm text-ink">
							<TruckIcon className="mt-0.5 size-5 shrink-0 text-success-600" />
							<span>{t("catalog.product.freight_placeholder", { uf: product.shop.ship_from_uf })}</span>
						</p>
						{product.category && (
							<p className="text-sm text-muted">
								{t("storefront.product.in_category")}{" "}
								<Link href={`/c/${product.category.slug}`} className="text-action-600 underline">
									{product.category.name}
								</Link>
							</p>
						)}
					</div>
				</div>

				<aside
					aria-label={t("storefront.product.buy_box")}
					className="flex flex-col gap-4 rounded-md border border-line p-4 lg:row-span-2 lg:self-start"
				>
					<div>
						<p className="font-semibold text-ink">
							{product.stock > 0
								? t("storefront.product.stock_available")
								: t("catalog.product.out_of_stock")}
						</p>
						{product.stock > 0 && (
							<p className="text-sm text-muted">
								{t("catalog.product.in_stock", { count: product.stock })}
							</p>
						)}
					</div>
					<AddToCartButton slug={product.slug} stock={product.stock} />
					<FreightBox slug={product.slug} />
					<section aria-labelledby="seller-title" className="border-t border-line pt-4 text-sm">
						<div className="flex items-center gap-3">
							<span className="flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-full border border-line bg-white text-action-600">
								{product.shop.logo_url ? (
									<img src={product.shop.logo_url} alt="" className="size-full object-cover" />
								) : (
									<StoreIcon className="size-6" />
								)}
							</span>
							<div className="min-w-0">
								<h2 id="seller-title" className="text-xs text-muted">
									{t("catalog.product.sold_by")}
								</h2>
								<Link
									href={`/s/${product.shop.slug}`}
									className="block truncate font-semibold text-action-600 hover:underline"
								>
									{product.shop.name}
								</Link>
							</div>
						</div>
						<p className="mt-2 text-muted">{t("catalog.product.seller_since", { year: since })}</p>
						<p className="text-muted">{t("catalog.card.ships_from", { uf: product.shop.ship_from_uf })}</p>
						<div className="mt-3">
							<ChatWithSeller slug={product.slug} own={qna.own_product} />
						</div>
					</section>
				</aside>

				<div className="space-y-8 border-t border-line pt-6">
					<section aria-labelledby="description-title">
						<h2 id="description-title" className="mb-3 text-xl font-semibold text-ink">
							{t("catalog.product.description")}
						</h2>
						<p className="max-w-prose break-words whitespace-pre-line text-muted">
							{product.description || t("catalog.product.no_description")}
						</p>
					</section>

					{reviews ? (
						<ReviewsSection block={reviews} />
					) : (
						<section aria-labelledby="reviews-title">
							<h2 id="reviews-title" className="mb-3 text-xl font-semibold text-ink">
								{t("storefront.product.reviews")}
							</h2>
							<p className="text-muted">{t("storefront.product.no_reviews")}</p>
						</section>
					)}

					<ProductQuestions slug={product.slug} questions={questions} qna={qna} />
				</div>
			</article>

			<Shelf
				id="more-title"
				title={t("catalog.product.more_from", { shop: product.shop.name })}
				products={more}
				seeAll={`/s/${product.shop.slug}`}
			/>
		</>
	);
}
