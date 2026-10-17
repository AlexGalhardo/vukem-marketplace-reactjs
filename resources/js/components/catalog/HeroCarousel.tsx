import { Link } from "@inertiajs/react";
import { type JSX, type ReactNode, useEffect, useState } from "react";

import { type MessageKey, useT } from "../../lib/i18n";
import { ChevronLeftIcon, ChevronRightIcon, PauseIcon, PlayIcon } from "../icons";

interface Slide {
	title: MessageKey;
	body: MessageKey;
	cta: MessageKey;
	href: string;
	tone: string;
	art: ReactNode;
}

// Our own banner art: flat shapes in the theme colours, no photos.
const parcels = (
	<g>
		<rect x="40" y="70" width="110" height="90" rx="8" className="fill-brand-400" />
		<path d="M40 100h110M95 70v30" className="stroke-banner-3" strokeOpacity=".25" strokeWidth="4" />
		<rect x="120" y="20" width="80" height="62" rx="8" className="fill-white" />
		<path d="M120 42h80M160 20v22" className="stroke-banner-1" strokeOpacity=".5" strokeWidth="4" />
		<rect x="160" y="96" width="70" height="64" rx="8" className="fill-banner-sky" />
	</g>
);
const tag = (
	<g>
		<path d="M60 40h90l60 60-90 90-60-60z" className="fill-white" />
		<circle cx="95" cy="75" r="12" className="fill-banner-2" />
		<path
			d="M110 120l40-40M115 85h.1M145 115h.1"
			className="stroke-banner-2"
			strokeWidth="10"
			strokeLinecap="round"
		/>
	</g>
);
const stall = (
	<g>
		<path d="M50 70l20-40h120l20 40z" className="fill-brand-400" />
		<path
			d="M50 70h160v20c0 10-10 16-20 16s-20-6-20-16c0 10-10 16-20 16s-20-6-20-16c0 10-10 16-20 16s-20-6-20-16c0 10-10 16-20 16s-20-6-20-16z"
			className="fill-white"
		/>
		<rect x="62" y="106" width="136" height="70" className="fill-white" fillOpacity=".85" />
		<rect x="110" y="126" width="40" height="50" rx="4" className="fill-banner-1" />
	</g>
);

const slides: Slide[] = [
	{
		title: "app.tagline",
		body: "home.hero_body",
		cta: "home.cta_browse",
		href: "/search",
		tone: "bg-banner-1 text-white",
		art: parcels,
	},
	{
		title: "storefront.banner.deals_title",
		body: "storefront.banner.deals_body",
		cta: "storefront.banner.deals_cta",
		href: "/search?sort=price_asc",
		tone: "bg-banner-2 text-white",
		art: tag,
	},
	{
		title: "storefront.banner.sell_title",
		body: "storefront.banner.sell_body",
		cta: "home.cta_sell",
		href: "/sell",
		tone: "bg-banner-3 text-white",
		art: stall,
	},
];

const interval = 7000;

// HeroCarousel is the home banner: three slides, arrows, dots, and
// auto-advance that pauses on hover/focus, on the pause button and under
// prefers-reduced-motion (WCAG 2.2.2).
export default function HeroCarousel(): JSX.Element {
	const { t } = useT();
	const [index, setIndex] = useState(0);
	const [paused, setPaused] = useState(false);
	const [hold, setHold] = useState(false);
	const [reduced, setReduced] = useState(false);

	useEffect(() => {
		const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
		const sync = (): void => setReduced(mq.matches);
		sync();
		mq.addEventListener("change", sync);
		return () => mq.removeEventListener("change", sync);
	}, []);

	const running = !paused && !hold && !reduced;
	useEffect(() => {
		if (!running) return;
		const timer = window.setInterval(() => setIndex((i) => (i + 1) % slides.length), interval);
		return () => window.clearInterval(timer);
	}, [running]);

	const go = (i: number): void => setIndex((i + slides.length) % slides.length);
	const nav =
		"absolute top-1/2 hidden size-11 -translate-y-1/2 items-center justify-center rounded-full bg-card/90 text-action-600 shadow-lift hover:bg-card sm:flex";

	return (
		<section
			aria-roledescription={t("storefront.banner.carousel")}
			aria-label={t("storefront.banner.label")}
			className="relative overflow-hidden rounded-md shadow-card"
			onMouseEnter={() => setHold(true)}
			onMouseLeave={() => setHold(false)}
			onFocus={() => setHold(true)}
			onBlur={(e) => {
				if (!e.currentTarget.contains(e.relatedTarget)) setHold(false);
			}}
		>
			{slides.map((s, i) => (
				// biome-ignore lint/a11y/useSemanticElements: APG carousel slides are role=group; a fieldset would announce a form.
				<div
					key={s.title}
					role="group"
					aria-roledescription={t("storefront.banner.slide")}
					aria-label={t("storefront.banner.position", { n: i + 1, total: slides.length })}
					hidden={i !== index}
					className={`${s.tone} grid h-56 grid-cols-[1fr_auto] items-center gap-4 px-6 sm:h-72 sm:px-16 lg:h-80`}
				>
					<div className="max-w-md">
						<p className="text-2xl leading-tight font-semibold text-balance sm:text-4xl">{t(s.title)}</p>
						<p className="mt-2 hidden text-base text-white/90 sm:block">{t(s.body)}</p>
						<Link
							href={s.href}
							className="mt-4 inline-flex h-10 items-center rounded-md bg-white px-5 text-sm font-semibold text-banner-1 hover:bg-white/90"
						>
							{t(s.cta)}
						</Link>
					</div>
					<svg viewBox="0 0 260 200" aria-hidden="true" className="hidden h-44 sm:block lg:h-56">
						{s.art}
					</svg>
				</div>
			))}
			<button
				type="button"
				onClick={() => go(index - 1)}
				aria-label={t("storefront.banner.previous")}
				className={`${nav} left-3`}
			>
				<ChevronLeftIcon className="size-6" />
			</button>
			<button
				type="button"
				onClick={() => go(index + 1)}
				aria-label={t("storefront.banner.next")}
				className={`${nav} right-3`}
			>
				<ChevronRightIcon className="size-6" />
			</button>
			<div className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-1">
				{slides.map((s, i) => (
					<button
						key={s.title}
						type="button"
						onClick={() => go(i)}
						aria-label={t("storefront.banner.position", { n: i + 1, total: slides.length })}
						aria-current={i === index ? "true" : undefined}
						className="flex size-6 items-center justify-center"
					>
						<span className={`block size-2 rounded-full ${i === index ? "bg-white" : "bg-white/50"}`} />
					</button>
				))}
				{!reduced && (
					<button
						type="button"
						onClick={() => setPaused((p) => !p)}
						aria-label={paused ? t("storefront.banner.play") : t("storefront.banner.pause")}
						className="ml-1 flex size-6 items-center justify-center rounded-full text-white"
					>
						{paused ? <PlayIcon className="size-4" /> : <PauseIcon className="size-4" />}
					</button>
				)}
			</div>
		</section>
	);
}
