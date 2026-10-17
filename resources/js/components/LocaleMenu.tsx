import { Menu } from "@base-ui/react/menu";
import { router, usePage } from "@inertiajs/react";
import type { JSX } from "react";

import { type MessageKey, useT } from "../lib/i18n";
import type { SharedProps } from "../types";

export default function LocaleMenu(): JSX.Element {
	const { t, locale } = useT();
	const { locales } = usePage<SharedProps>().props;

	const change = (value: unknown) => {
		if (typeof value === "string" && value !== locale) {
			router.post("/locale", { locale: value }, { preserveScroll: true });
		}
	};

	return (
		<Menu.Root>
			<Menu.Trigger
				aria-label={t("locale.label")}
				data-testid="locale-menu"
				className="flex h-9 items-center gap-1 rounded-full px-3 text-sm font-medium text-brand-900 hover:bg-black/10 dark:text-ink dark:hover:bg-white/10"
			>
				{locale === "pt-BR" ? "PT" : "EN"}
				<svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
					<path d="M4 6l4 4 4-4z" />
				</svg>
			</Menu.Trigger>
			<Menu.Portal>
				<Menu.Positioner sideOffset={6} align="end" className="outline-hidden">
					<Menu.Popup className="min-w-44 rounded-md border border-line bg-card py-1 text-ink shadow-lg outline-hidden">
						<Menu.RadioGroup value={locale} onValueChange={change}>
							{locales.map((code) => (
								<Menu.RadioItem
									key={code}
									value={code}
									closeOnClick
									className="grid cursor-default grid-cols-[1rem_1fr] items-center gap-2 px-3 py-2 text-sm outline-hidden select-none data-highlighted:bg-action-500 data-highlighted:text-white"
								>
									<Menu.RadioItemIndicator className="col-start-1">✓</Menu.RadioItemIndicator>
									<span className="col-start-2">{t(`locale.${code}` as MessageKey)}</span>
								</Menu.RadioItem>
							))}
						</Menu.RadioGroup>
					</Menu.Popup>
				</Menu.Positioner>
			</Menu.Portal>
		</Menu.Root>
	);
}
