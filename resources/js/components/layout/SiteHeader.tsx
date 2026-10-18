import { Link, usePage } from "@inertiajs/react";
import type { JSX } from "react";
import { formatCep, useSavedCep } from "../../lib/cep";
import { useT } from "../../lib/i18n";
import type { SharedProps } from "../../types";
import { CartIcon, ChatIcon, PinIcon, SearchIcon, UserIcon } from "../icons";
import LocaleMenu from "../LocaleMenu";
import ThemeToggle from "../ThemeToggle";
import Logo from "./Logo";

const pill = "shrink-0 whitespace-nowrap rounded-full px-2.5 py-1.5 hover:bg-header-hover";

function CartLink({ count, className = "" }: { count: number; className?: string }): JSX.Element {
	const { t } = useT();
	return (
		<Link
			href="/cart"
			className={`relative flex size-10 shrink-0 items-center justify-center rounded-full hover:bg-header-hover ${className}`}
		>
			<CartIcon className="size-6" />
			<span className="sr-only">
				{t("nav.cart")}
				{count > 0 ? ` (${t("storefront.header.cart_count", { count })})` : ""}
			</span>
			{count > 0 && (
				<span
					aria-hidden="true"
					className="absolute top-0.5 right-0 min-w-4.5 rounded-full bg-action-500 px-1 text-center text-[0.7rem] leading-4.5 font-semibold text-on-action tabular-nums"
				>
					{count > 99 ? "99+" : count}
				</span>
			)}
		</Link>
	);
}

// DeliverTo is the "Deliver to …" line: the CEP last used in the freight
// box, or an invitation to enter one.
function DeliverTo({ className = "" }: { className?: string }): JSX.Element {
	const { t } = useT();
	const cep = useSavedCep();
	const signedIn = Boolean(usePage<SharedProps>().props.auth?.user);

	return (
		<Link
			href={signedIn ? "/account/addresses" : "/login"}
			className={`flex shrink-0 items-center gap-1.5 rounded-md py-1 pr-2 hover:bg-header-hover ${className}`}
		>
			<PinIcon className="size-5 text-header-muted" />
			<span className="flex flex-col leading-tight">
				<span className="text-[0.7rem] text-header-muted">{t("storefront.header.deliver_to")}</span>
				<span className="text-sm">{cep ? formatCep(cep) : t("storefront.header.enter_cep")}</span>
			</span>
		</Link>
	);
}

// SiteHeader is the yellow top bar: logo, big search, the delivery line,
// shop shortcuts and the account/cart links. On phones the shortcut and
// account links share one horizontally scrolling row.
export default function SiteHeader(): JSX.Element {
	const page = usePage<SharedProps>();
	const { t } = useT();
	const user = page.props.auth?.user ?? null;
	const hasShop = page.props.auth?.hasShop ?? false;
	const cartCount = page.props.cartCount ?? 0;
	const chat = page.props.chat;
	const q = typeof window === "undefined" ? "" : (new URLSearchParams(window.location.search).get("q") ?? "");

	return (
		<header className="bg-header text-on-header shadow-card">
			<a
				href="#main"
				className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-card focus:px-4 focus:py-2 focus:text-ink focus:shadow-lift"
			>
				{t("storefront.header.skip")}
			</a>
			<div className="mx-auto max-w-6xl px-4">
				<div className="flex items-center gap-3 pt-2.5 pb-2 md:gap-8">
					<Link href="/" className="shrink-0 rounded-md">
						<Logo name={t("app.name")} compact />
					</Link>
					<search className="min-w-0 flex-1 md:max-w-xl">
						<form action="/search" method="get" className="flex h-10 rounded-sm bg-card shadow-card">
							<label htmlFor="q" className="sr-only">
								{t("nav.search")}
							</label>
							<input
								id="q"
								name="q"
								type="search"
								defaultValue={q}
								key={q}
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
						</form>
					</search>
					<CartLink count={cartCount} className="md:hidden" />
					{/* Top row, not the link row: axe spots the open (modal) locale menu by sampling
					    points that must all hit its backdrop, which is cut out around this trigger. */}
					<div className="flex shrink-0 items-center md:ml-auto">
						<LocaleMenu />
						<ThemeToggle />
					</div>
				</div>

				<DeliverTo className="mb-1 md:hidden" />

				<div className="relative -mx-4 flex items-center gap-1 overflow-x-auto px-4 pb-1.5 text-sm scrollbar-none md:mx-0 md:gap-4 md:overflow-visible md:px-0">
					<DeliverTo className="hidden md:flex md:w-44" />
					<nav aria-label={t("storefront.header.nav_label")} className="flex shrink-0 items-center gap-1">
						<Link href="/search" className={pill}>
							{t("nav.categories")}
						</Link>
						<Link href="/search?sort=price_asc" className={pill}>
							{t("nav.offers")}
						</Link>
						<Link href="/search?sort=newest" className={pill}>
							{t("storefront.header.new")}
						</Link>
						<Link href={hasShop ? "/seller" : "/sell"} className={pill}>
							{t("nav.sell")}
						</Link>
						<Link href="/contact" className={pill}>
							{t("storefront.header.help")}
						</Link>
					</nav>
					<div className="ml-auto flex shrink-0 items-center gap-1">
						{user ? (
							<>
								<Link href="/dashboard" className={`${pill} flex items-center gap-1.5`}>
									<UserIcon className="size-4.5" />
									<span className="max-w-40 truncate">{user.name}</span>
								</Link>
								{chat?.enabled && (
									<Link href="/messages" className={`${pill} flex items-center gap-1.5`}>
										<ChatIcon className="size-4.5" />
										<span aria-hidden={chat.unread > 0 ? true : undefined}>{t("chat.nav")}</span>
										{chat.unread > 0 && (
											<>
												<span className="sr-only">
													{t("chat.nav_unread", { count: chat.unread })}
												</span>
												<span
													aria-hidden="true"
													data-testid="chat-unread"
													className="min-w-4.5 rounded-full bg-action-500 px-1 text-center text-[0.7rem] leading-4.5 font-semibold text-on-action tabular-nums"
												>
													{chat.unread > 99 ? "99+" : chat.unread}
												</span>
											</>
										)}
									</Link>
								)}
								<Link href="/account" className={pill}>
									{t("account.nav.account")}
								</Link>
								<Link href="/logout" method="post" as="button" className={pill}>
									{t("nav.logout")}
								</Link>
							</>
						) : (
							<>
								<Link href="/register" className={pill}>
									{t("nav.register")}
								</Link>
								<Link href="/login" className={pill}>
									{t("nav.login")}
								</Link>
							</>
						)}
						<CartLink count={cartCount} className="hidden md:flex" />
					</div>
				</div>
			</div>
		</header>
	);
}
