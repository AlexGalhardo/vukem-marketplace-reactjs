// Shared Tailwind class lists for the account pages, built on the theme
// tokens in resources/css/app.css (light and dark).
export const card = "mx-auto mt-2 w-full max-w-md rounded-md bg-card p-6 text-ink shadow-card sm:mt-6 sm:p-8";
export const panel = "rounded-md bg-card p-6 text-ink shadow-card";
export const heading = "mb-4 text-2xl font-semibold text-ink";
export const subheading = "mb-3 text-lg font-semibold text-ink";
export const muted = "text-sm text-muted";
export const button =
	"inline-flex h-11 items-center justify-center rounded-md bg-action-500 px-5 text-sm font-semibold text-on-action hover:bg-action-600 disabled:opacity-50";
export const secondaryButton =
	"inline-flex h-11 items-center justify-center rounded-md bg-action-soft px-5 text-sm font-semibold text-action-600 hover:bg-action-soft-hover disabled:opacity-50";
export const dangerButton =
	"inline-flex h-11 items-center justify-center rounded-md bg-danger-600 px-5 text-sm font-semibold text-on-status hover:opacity-90 disabled:opacity-50";
export const link = "text-action-600 underline-offset-2 hover:underline";
