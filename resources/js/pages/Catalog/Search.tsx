import { Head, Link, router } from "@inertiajs/react";
import { type FormEvent, type JSX, useEffect, useState } from "react";
import Pagination from "../../components/catalog/Pagination";
import type { CardLayout } from "../../components/catalog/ProductCard";
import ProductGrid from "../../components/catalog/ProductGrid";
import { FilterIcon, GridIcon, ListIcon } from "../../components/icons";
import { type MessageKey, useT } from "../../lib/i18n";
import { centsToInput, formatBRL, toCents } from "../../lib/money";
import type { CategoryRef, Page, ProductCard, SearchQuery } from "../../types";

interface Props {
	query: SearchQuery;
	results: Page<ProductCard>;
	category: CategoryRef | null;
	categories: CategoryRef[];
	ufs: string[];
}

const sorts: { value: SearchQuery["sort"]; label: MessageKey }[] = [
	{ value: "relevance", label: "catalog.search.sort_relevance" },
	{ value: "price_asc", label: "catalog.search.sort_price_asc" },
	{ value: "price_desc", label: "catalog.search.sort_price_desc" },
	{ value: "newest", label: "catalog.search.sort_newest" },
];

// Price shortcuts in cents (0 = open end).
const ranges: { min: number; max: number }[] = [
	{ min: 0, max: 10000 },
	{ min: 10000, max: 30000 },
	{ min: 30000, max: 0 },
];

const inputClass =
	"h-9 w-full rounded-md border border-line bg-card px-2 text-sm text-ink focus:border-action-500 focus:outline-none";
const filterLink = "text-left text-sm text-muted hover:text-ink";
const viewKey = "results_view";

function readView(): CardLayout {
	try {
		return localStorage.getItem(viewKey) === "grid" ? "grid" : "row";
	} catch {
		return "row";
	}
}

export default function Search({ query, results, category, categories, ufs }: Props): JSX.Element {
	const { t, locale } = useT();
	const [min, setMin] = useState(query.min ? centsToInput(query.min) : "");
	const [max, setMax] = useState(query.max ? centsToInput(query.max) : "");
	const [showFilters, setShowFilters] = useState(false);
	const [view, setView] = useState<CardLayout>("row");
	const path = category ? `/c/${category.slug}` : "/search";

	useEffect(() => setView(readView()), []);
	const chooseView = (v: CardLayout): void => {
		setView(v);
		try {
			localStorage.setItem(viewKey, v);
		} catch {
			// Storage blocked: the choice lasts for this page view.
		}
	};

	// Filters live in the URL, so results are shareable and the back button works.
	const visit = (changes: Partial<Record<keyof SearchQuery, string | number>>): void => {
		const params: Record<string, string> = {};
		const merged = { q: query.q, uf: query.uf, sort: query.sort, min: query.min, max: query.max, ...changes };
		for (const [k, v] of Object.entries(merged)) {
			if (v !== "" && v !== 0 && v !== undefined && !(k === "sort" && v === "relevance")) params[k] = String(v);
		}
		router.get(path, params, { preserveState: true });
	};

	const applyPrice = (e: FormEvent): void => {
		e.preventDefault();
		const minCents = min ? toCents(min) : 0;
		const maxCents = max ? toCents(max) : 0;
		visit({ min: Number.isNaN(minCents) ? 0 : minCents, max: Number.isNaN(maxCents) ? 0 : maxCents });
	};

	const rangeLabel = (r: { min: number; max: number }): string =>
		r.min === 0
			? t("storefront.search.up_to", { max: formatBRL(r.max, locale) })
			: r.max === 0
				? t("storefront.search.over", { min: formatBRL(r.min, locale) })
				: t("storefront.search.between", { min: formatBRL(r.min, locale), max: formatBRL(r.max, locale) });

	const heading = category
		? category.name
		: query.q
			? t("catalog.search.results_for", { q: query.q })
			: t("catalog.search.all_products");
	const qs = query.q ? `?q=${encodeURIComponent(query.q)}` : "";

	const chips: { label: string; clear: () => void }[] = [];
	if (query.min || query.max) {
		chips.push({
			label: rangeLabel({ min: query.min, max: query.max }),
			clear: () => {
				setMin("");
				setMax("");
				visit({ min: 0, max: 0 });
			},
		});
	}
	if (query.uf) {
		chips.push({ label: t("catalog.card.ships_from", { uf: query.uf }), clear: () => visit({ uf: "" }) });
	}

	return (
		<>
			<Head title={heading} />
			<div className="grid gap-4 md:grid-cols-[15rem_1fr] md:gap-6">
				<aside aria-label={t("catalog.search.filters")} className="space-y-5">
					<div>
						<h1 className="text-2xl leading-tight font-semibold break-words text-ink">{heading}</h1>
						<p className="mt-1 text-sm text-muted">{t("catalog.search.count", { count: results.total })}</p>
					</div>

					<button
						type="button"
						aria-expanded={showFilters}
						aria-controls="search-filters"
						onClick={() => setShowFilters((s) => !s)}
						className="flex h-10 w-full items-center justify-center gap-2 rounded-md bg-card text-sm font-medium text-action-600 shadow-card md:hidden"
					>
						<FilterIcon className="size-4.5" />
						{t("catalog.search.filters")}
					</button>

					<div id="search-filters" className={`space-y-6 ${showFilters ? "" : "hidden md:block"}`}>
						{chips.length > 0 && (
							<section aria-labelledby="applied-title">
								<h2 id="applied-title" className="mb-2 text-sm font-semibold text-ink">
									{t("storefront.search.applied")}
								</h2>
								<ul className="flex flex-wrap gap-2">
									{chips.map((c) => (
										<li key={c.label}>
											<button
												type="button"
												onClick={c.clear}
												aria-label={t("storefront.search.remove_filter", { filter: c.label })}
												className="flex items-center gap-1.5 rounded-full bg-card px-3 py-1 text-xs text-ink shadow-card hover:text-action-600"
											>
												{c.label}
												<span aria-hidden="true">×</span>
											</button>
										</li>
									))}
								</ul>
							</section>
						)}

						<nav aria-label={t("nav.categories")}>
							<h2 className="mb-2 text-base font-semibold text-ink">{t("nav.categories")}</h2>
							<ul className="space-y-1.5">
								{category && (
									<li>
										<Link href={`/search${qs}`} className="text-sm text-action-600 hover:underline">
											{t("storefront.search.all_categories")}
										</Link>
									</li>
								)}
								{categories.map((c) => (
									<li key={c.slug}>
										<Link
											href={`/c/${c.slug}${qs}`}
											aria-current={c.slug === category?.slug ? "page" : undefined}
											className={`${filterLink} aria-[current=page]:font-semibold aria-[current=page]:text-ink`}
										>
											{c.name}
										</Link>
									</li>
								))}
							</ul>
						</nav>

						<section aria-labelledby="price-title">
							<h2 id="price-title" className="mb-2 text-base font-semibold text-ink">
								{t("catalog.search.price")}
							</h2>
							<ul className="mb-3 space-y-1.5">
								{ranges.map((r) => (
									<li key={`${r.min}-${r.max}`}>
										<button
											type="button"
											aria-pressed={query.min === r.min && query.max === r.max}
											onClick={() => {
												setMin(r.min ? centsToInput(r.min) : "");
												setMax(r.max ? centsToInput(r.max) : "");
												visit({ min: r.min, max: r.max });
											}}
											className={`${filterLink} aria-pressed:font-semibold aria-pressed:text-ink`}
										>
											{rangeLabel(r)}
										</button>
									</li>
								))}
							</ul>
							<form onSubmit={applyPrice} className="space-y-2">
								<fieldset>
									<legend className="sr-only">{t("catalog.search.price")}</legend>
									<div className="flex items-center gap-2">
										<label htmlFor="min" className="sr-only">
											{t("catalog.search.min")}
										</label>
										<input
											id="min"
											inputMode="decimal"
											maxLength={12}
											placeholder={t("catalog.search.min")}
											value={min}
											onChange={(e) => setMin(e.target.value)}
											className={inputClass}
										/>
										<span aria-hidden="true" className="text-muted">
											–
										</span>
										<label htmlFor="max" className="sr-only">
											{t("catalog.search.max")}
										</label>
										<input
											id="max"
											inputMode="decimal"
											maxLength={12}
											placeholder={t("catalog.search.max")}
											value={max}
											onChange={(e) => setMax(e.target.value)}
											className={inputClass}
										/>
									</div>
								</fieldset>
								<button
									type="submit"
									className="h-9 w-full rounded-md bg-action-soft px-3 text-sm font-medium text-action-600 hover:bg-action-soft-hover"
								>
									{t("catalog.search.apply")}
								</button>
							</form>
						</section>

						<div>
							<label htmlFor="uf" className="mb-2 block text-base font-semibold text-ink">
								{t("catalog.search.ships_from")}
							</label>
							<select
								id="uf"
								value={query.uf}
								onChange={(e) => visit({ uf: e.target.value })}
								className={inputClass}
							>
								<option value="">{t("catalog.search.any_state")}</option>
								{ufs.map((uf) => (
									<option key={uf} value={uf}>
										{uf}
									</option>
								))}
							</select>
						</div>
					</div>
				</aside>

				<div className="min-w-0">
					<div className="mb-3 flex flex-wrap items-center justify-end gap-3">
						<div className="flex items-center gap-2">
							<label htmlFor="sort" className="text-sm font-medium whitespace-nowrap text-ink">
								{t("catalog.search.sort")}
							</label>
							<select
								id="sort"
								value={query.sort}
								onChange={(e) => visit({ sort: e.target.value })}
								className="h-9 rounded-md border-0 bg-transparent pr-7 text-sm font-medium text-action-600 focus:outline-none"
							>
								{sorts.map((s) => (
									<option key={s.value} value={s.value}>
										{t(s.label)}
									</option>
								))}
							</select>
						</div>
						<fieldset className="flex overflow-hidden rounded-md bg-card shadow-card">
							<legend className="sr-only">{t("storefront.search.view")}</legend>
							{(
								[
									["row", ListIcon, "storefront.search.view_list"],
									["grid", GridIcon, "storefront.search.view_grid"],
								] as const
							).map(([v, ViewIcon, label]) => (
								<button
									key={v}
									type="button"
									aria-pressed={view === v}
									aria-label={t(label)}
									onClick={() => chooseView(v)}
									className="flex size-9 items-center justify-center text-muted hover:text-ink aria-pressed:bg-action-soft aria-pressed:text-action-600"
								>
									<ViewIcon className="size-5" />
								</button>
							))}
						</fieldset>
					</div>
					<ProductGrid
						products={results.data}
						layout={view}
						columns="narrow"
						empty={
							<div className="rounded-md bg-card px-6 py-12 text-center shadow-card">
								<p className="text-lg font-semibold text-ink">{t("catalog.search.empty_title")}</p>
								<p className="mt-1 text-sm text-muted">{t("catalog.search.empty_body")}</p>
							</div>
						}
					/>
					<Pagination page={results.page} lastPage={results.last_page} />
				</div>
			</div>
		</>
	);
}
