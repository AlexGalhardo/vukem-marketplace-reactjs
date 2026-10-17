import { Toggle } from "@base-ui/react/toggle";
import type { JSX } from "react";

import { useT } from "../lib/i18n";
import { useTheme } from "../lib/theme";

export default function ThemeToggle(): JSX.Element {
	const { t } = useT();
	const { theme, setTheme } = useTheme();

	return (
		<Toggle
			aria-label={t("theme.toggle")}
			title={theme === "dark" ? t("theme.light") : t("theme.dark")}
			pressed={theme === "dark"}
			onPressedChange={(pressed) => setTheme(pressed ? "dark" : "light")}
			data-testid="theme-toggle"
			className="flex size-9 items-center justify-center rounded-full text-brand-900 hover:bg-black/10 dark:text-ink dark:hover:bg-white/10"
		>
			{theme === "dark" ? <MoonIcon /> : <SunIcon />}
		</Toggle>
	);
}

function SunIcon(): JSX.Element {
	return (
		<svg
			width="18"
			height="18"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<circle cx="12" cy="12" r="4" />
			<path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
		</svg>
	);
}

function MoonIcon(): JSX.Element {
	return (
		<svg
			width="18"
			height="18"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
		</svg>
	);
}
