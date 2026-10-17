import { Head, router } from "@inertiajs/react";
import { type FormEvent, type JSX, useState } from "react";

import CategoryRail from "../../components/catalog/CategoryRail";
import Pagination from "../../components/catalog/Pagination";
import ProductGrid from "../../components/catalog/ProductGrid";
import { type MessageKey, useT } from "../../lib/i18n";
import { centsToInput, toCents } from "../../lib/money";
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

const inputClass =
	"h-9 w-full rounded-md border border-line bg-card px-2 text-sm text-ink focus:border-action-500 focus:outline-none";

export default function Search({ query, results, category, categories, ufs }: Props): JSX.Element {
	const { t } = useT();
	const [min, setMin] = useState(query.min ? centsToInput(query.min) : "");
	const [max, setMax] = useState(query.max ? centsToInput(query.max) : "");
	const path = category ? `/c/${category.slug}` : "/search";

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

	const heading = category
		? category.name
		: query.q
			? t("catalog.search.results_for", { q: query.q })
			: t("catalog.search.all_products");

	return (
		<>
			<Head title={heading} />
			<div className="mb-4">
				<CategoryRail categories={categories} active={category?.slug} label={t("nav.categories")} />
			</div>
			<div className="grid gap-6 md:grid-cols-[14rem_1fr]">
				<aside aria-label={t("catalog.search.filters")} className="space-y-5">
					<div>
						<h1 className="text-xl font-semibold text-ink">{heading}</h1>
						<p className="text-sm text-muted">{t("catalog.search.count", { count: results.total })}</p>
					</div>
					<div>
						<label htmlFor="sort" className="mb-1 block text-sm font-medium text-ink">
							{t("catalog.search.sort")}
						</label>
						<select
							id="sort"
							value={query.sort}
							onChange={(e) => visit({ sort: e.target.value })}
							className={inputClass}
						>
							{sorts.map((s) => (
								<option key={s.value} value={s.value}>
									{t(s.label)}
								</option>
							))}
						</select>
					</div>
					<form onSubmit={applyPrice} className="space-y-2">
						<fieldset>
							<legend className="mb-1 text-sm font-medium text-ink">{t("catalog.search.price")}</legend>
							<div className="flex items-center gap-2">
								<label htmlFor="min" className="sr-only">
									{t("catalog.search.min")}
								</label>
								<input
									id="min"
									inputMode="decimal"
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
									placeholder={t("catalog.search.max")}
									value={max}
									onChange={(e) => setMax(e.target.value)}
									className={inputClass}
								/>
							</div>
						</fieldset>
						<button
							type="submit"
							className="w-full rounded-md border border-action-600 px-3 py-1.5 text-sm text-action-600 hover:bg-action-500/10"
						>
							{t("catalog.search.apply")}
						</button>
					</form>
					<div>
						<label htmlFor="uf" className="mb-1 block text-sm font-medium text-ink">
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
				</aside>
				<div>
					<ProductGrid
						products={results.data}
						empty={
							<div className="rounded-md bg-card px-6 py-10 text-center shadow-sm">
								<p className="font-medium text-ink">{t("catalog.search.empty_title")}</p>
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
