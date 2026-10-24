import { router } from "@inertiajs/react";
import { type JSX, type KeyboardEvent, useEffect, useId, useRef, useState } from "react";
import { useT } from "../../lib/i18n";
import { formatBRL } from "../../lib/money";
import { SearchIcon } from "../icons";

const MIN_CHARS = 3;
const MAX_CHARS = 100;
const DEBOUNCE_MS = 200;

interface Suggestion {
	slug: string;
	title: string;
	category: string;
	image_url: string;
	price_cents: number;
}

// SearchBox is the navbar search: a plain GET form to /search that also
// suggests the best 8 products from MIN_CHARS characters on (a combobox:
// arrows move, Enter opens the active product, Escape closes).
export default function SearchBox({ initial }: { initial: string }): JSX.Element {
	const { t, locale } = useT();
	const listId = useId();
	const [value, setValue] = useState(initial);
	const [items, setItems] = useState<Suggestion[]>([]);
	const [open, setOpen] = useState(false);
	const [active, setActive] = useState(-1);
	const root = useRef<HTMLFormElement>(null);
	const term = value.trim();

	useEffect(() => {
		if (term.length < MIN_CHARS) {
			setItems([]);
			return;
		}
		const ctrl = new AbortController();
		const timer = setTimeout(() => {
			fetch(`/search/suggest?q=${encodeURIComponent(term)}`, {
				signal: ctrl.signal,
				headers: { Accept: "application/json" },
			})
				.then((res) => (res.ok ? (res.json() as Promise<Suggestion[]>) : []))
				.then((data) => {
					setItems(data);
					setActive(-1);
				})
				.catch(() => {});
		}, DEBOUNCE_MS);
		return () => {
			clearTimeout(timer);
			ctrl.abort();
		};
	}, [term]);

	useEffect(() => {
		const close = (e: MouseEvent): void => {
			if (root.current && !root.current.contains(e.target as Node)) setOpen(false);
		};
		document.addEventListener("mousedown", close);
		return () => document.removeEventListener("mousedown", close);
	}, []);

	const showList = open && term.length >= MIN_CHARS && items.length > 0;

	const onKeyDown = (e: KeyboardEvent<HTMLInputElement>): void => {
		if (e.key === "Escape") {
			setOpen(false);
		} else if (e.key === "ArrowDown" && items.length > 0) {
			e.preventDefault();
			setOpen(true);
			setActive((i) => (i + 1) % items.length);
		} else if (e.key === "ArrowUp" && items.length > 0) {
			e.preventDefault();
			setActive((i) => (i <= 0 ? items.length - 1 : i - 1));
		} else if (e.key === "Enter" && showList && active >= 0) {
			e.preventDefault();
			router.visit(`/p/${items[active]?.slug}`);
		}
	};

	return (
		<form
			ref={root}
			action="/search"
			method="get"
			className="relative flex h-10 rounded-sm bg-card shadow-card"
			onSubmit={() => setOpen(false)}
		>
			<label htmlFor="q" className="sr-only">
				{t("nav.search")}
			</label>
			<input
				id="q"
				name="q"
				type="search"
				role="combobox"
				aria-expanded={showList}
				aria-controls={listId}
				aria-autocomplete="list"
				aria-activedescendant={showList && active >= 0 ? `${listId}-${active}` : undefined}
				autoComplete="off"
				maxLength={MAX_CHARS}
				value={value}
				onChange={(e) => {
					setValue(e.target.value);
					setOpen(true);
				}}
				onFocus={() => setOpen(true)}
				onKeyDown={onKeyDown}
				placeholder={t("nav.search_placeholder")}
				className="h-full min-w-0 flex-1 rounded-l-sm border-0 bg-transparent px-3.5 text-[0.95rem] text-ink placeholder:text-muted focus:outline-none"
			/>
			<button
				type="submit"
				className="flex h-full w-11 shrink-0 items-center justify-center rounded-r-sm text-muted hover:text-ink"
			>
				<span aria-hidden="true" className="h-6 w-px bg-line" />
				<SearchIcon className="mx-auto size-5" />
				<span className="sr-only">{t("nav.search")}</span>
			</button>
			{showList && (
				<div
					id={listId}
					role="listbox"
					aria-label={t("nav.search_suggestions")}
					className="absolute top-full right-0 left-0 z-40 mt-1 max-h-[28rem] overflow-y-auto rounded-md border border-line bg-card py-1 text-ink shadow-lift"
				>
					{items.map((p, i) => (
						<div
							key={p.slug}
							id={`${listId}-${i}`}
							role="option"
							tabIndex={-1}
							aria-selected={i === active}
						>
							<a
								href={`/p/${p.slug}`}
								onClick={(e) => {
									e.preventDefault();
									setOpen(false);
									router.visit(`/p/${p.slug}`);
								}}
								onMouseEnter={() => setActive(i)}
								className={`flex items-center gap-3 px-3 py-2 ${i === active ? "bg-header-hover" : ""}`}
							>
								<img
									src={p.image_url}
									alt=""
									width={36}
									height={36}
									loading="lazy"
									className="size-9 shrink-0 rounded-sm bg-white object-contain"
								/>
								<span className="flex min-w-0 flex-1 flex-col leading-tight">
									<span className="truncate text-sm font-medium">{p.title}</span>
									<span className="truncate text-xs text-muted">{p.category}</span>
								</span>
								<span className="shrink-0 text-sm font-semibold tabular-nums">
									{formatBRL(p.price_cents, locale)}
								</span>
							</a>
						</div>
					))}
				</div>
			)}
		</form>
	);
}
