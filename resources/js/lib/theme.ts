import { useCallback, useState } from "react";

export type Theme = "light" | "dark";

function current(): Theme {
	return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

// The initial class is applied by the inline script in app.html before
// paint, so there is no flash of the wrong theme.
export function useTheme(): { theme: Theme; setTheme: (theme: Theme) => void } {
	const [theme, setState] = useState<Theme>(current);

	const setTheme = useCallback((next: Theme) => {
		document.documentElement.classList.toggle("dark", next === "dark");
		try {
			localStorage.setItem("theme", next);
		} catch {
			// Storage can be blocked (private mode); the toggle still works for this page view.
		}
		setState(next);
	}, []);

	return { theme, setTheme };
}
