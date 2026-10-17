// Shared Tailwind class lists for the account pages, built on the theme
// tokens in resources/css/app.css (light and dark).
export const card = "mx-auto w-full max-w-md rounded-lg bg-card p-6 text-ink shadow-sm";
export const panel = "rounded-lg bg-card p-6 text-ink shadow-sm";
export const heading = "mb-4 text-2xl font-semibold text-ink";
export const subheading = "mb-3 text-lg font-semibold text-ink";
export const muted = "text-sm text-muted";
export const button =
	"inline-flex h-10 items-center justify-center rounded-md bg-action-600 px-4 text-sm font-medium text-on-action hover:bg-action-700 disabled:opacity-50";
export const secondaryButton =
	"inline-flex h-10 items-center justify-center rounded-md border border-line bg-card px-4 text-sm font-medium text-ink hover:bg-surface disabled:opacity-50";
export const dangerButton =
	"inline-flex h-10 items-center justify-center rounded-md bg-danger-600 px-4 text-sm font-medium text-on-status hover:opacity-90 disabled:opacity-50";
export const link = "text-action-600 underline-offset-2 hover:underline";
