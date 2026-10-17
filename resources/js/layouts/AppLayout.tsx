import { CSPProvider } from "@base-ui/react/csp-provider";
import { Link, usePage } from "@inertiajs/react";
import { type JSX, type ReactNode, useEffect } from "react";

import LocaleMenu from "../components/LocaleMenu";
import ThemeToggle from "../components/ThemeToggle";
import { useT } from "../lib/i18n";
import type { SharedProps } from "../types";

const nonce = document.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]')?.nonce;

export default function AppLayout({ children }: { children: ReactNode }): JSX.Element {
	const page = usePage<SharedProps>();
	const { t, locale } = useT();

	// The server sets <html lang> on first visits; Inertia visits keep it in sync.
	useEffect(() => {
		document.documentElement.lang = locale;
	}, [locale]);
	const message = page.flash?.success ?? page.flash?.status;

	return (
		<CSPProvider nonce={nonce}>
			<header className="bg-brand-400 dark:border-b dark:border-line dark:bg-card">
				<div className="mx-auto flex max-w-6xl flex-wrap items-center gap-3 px-4 py-3">
					<Link href="/" className="text-lg font-extrabold tracking-tight text-brand-900 dark:text-ink">
						{t("app.name")}
					</Link>
					<search className="order-last w-full sm:order-none sm:flex-1">
						<form action="/search" method="get" className="flex">
							<label htmlFor="q" className="sr-only">
								{t("nav.search")}
							</label>
							<input
								id="q"
								name="q"
								type="search"
								placeholder={t("nav.search_placeholder")}
								className="h-10 w-full rounded-l-sm border-0 bg-card px-3 text-sm text-ink shadow-sm placeholder:text-muted"
							/>
							<button
								type="submit"
								className="h-10 rounded-r-sm border-l border-line bg-card px-4 text-sm text-muted hover:text-ink"
							>
								{t("nav.search")}
							</button>
						</form>
					</search>
					<nav className="ml-auto flex items-center gap-1 text-sm text-brand-900 dark:text-ink">
						{page.props.auth?.user ? (
							<>
								<Link
									href="/dashboard"
									className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
								>
									{page.props.auth.user.name}
								</Link>
								<Link
									href="/account"
									className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
								>
									{t("account.nav.account")}
								</Link>
								<Link
									href="/cart"
									className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
								>
									{t("nav.cart")}
									{(page.props.cartCount ?? 0) > 0 && (
										<span className="ml-1 rounded-full bg-action-500 px-1.5 text-xs font-semibold text-on-action tabular-nums">
											{page.props.cartCount}
										</span>
									)}
								</Link>
								<Link
									href="/logout"
									method="post"
									as="button"
									className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
								>
									{t("nav.logout")}
								</Link>
							</>
						) : (
							<>
								<Link
									href="/register"
									className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
								>
									{t("nav.register")}
								</Link>
								<Link
									href="/login"
									className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
								>
									{t("nav.login")}
								</Link>
							</>
						)}
						<Link
							href={page.props.auth?.user ? "/seller" : "/sell"}
							className="rounded-full px-3 py-2 hover:bg-black/10 dark:hover:bg-white/10"
						>
							{t("nav.sell")}
						</Link>
						<LocaleMenu />
						<ThemeToggle />
					</nav>
				</div>
			</header>
			<main className="mx-auto max-w-6xl px-4 py-6">
				{typeof message === "string" && (
					<p role="status" className="mb-4 rounded-md bg-action-500/10 px-4 py-3 text-ink">
						{message}
					</p>
				)}
				{children}
			</main>
			<footer className="border-t border-line bg-card">
				<div className="mx-auto max-w-6xl px-4 py-6 text-xs text-muted">
					<p>{t("footer.rights", { year: new Date().getFullYear() })}</p>
					<p>{t("footer.not_affiliated")}</p>
				</div>
			</footer>
		</CSPProvider>
	);
}
