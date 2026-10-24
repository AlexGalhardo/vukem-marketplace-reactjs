import { createInertiaApp } from "@inertiajs/react";

import AppLayout from "./layouts/AppLayout";
import { installClientLogger } from "./lib/clientLog";

installClientLogger();

// The @inertiajs/vite plugin resolves pages from ./pages and mounts the app.
// The nonce lets the progress bar styles through the CSP.
createInertiaApp({
	layout: () => AppLayout,
	nonce: document.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]')?.nonce,
	progress: { color: "#2968c8" },
});
