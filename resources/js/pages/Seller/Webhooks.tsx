import { Head, Link, router, useForm } from "@inertiajs/react";
import { type FormEvent, type JSX, useState } from "react";

import Field from "../../components/Field";
import { useT } from "../../lib/i18n";
import { eventLabel } from "../../lib/webhooks";
import type { WebhookEndpoint, WebhookEvent } from "../../types";

interface Props {
	hasShop: boolean;
	endpoints: WebhookEndpoint[];
	events: WebhookEvent[];
	maxCount: number;
	secret: string | null;
}

function SecretPanel({ secret }: { secret: string }): JSX.Element | null {
	const { t } = useT();
	const [copied, setCopied] = useState(false);
	const [open, setOpen] = useState(true);

	if (!open) return null;

	const copy = async () => {
		try {
			await navigator.clipboard.writeText(secret);
			setCopied(true);
		} catch {
			setCopied(false);
		}
	};

	return (
		<section
			aria-labelledby="secret-title"
			className="mb-6 rounded-lg border-2 border-brand-400 bg-brand-50 px-5 py-4 text-brand-900 dark:bg-card dark:text-ink"
		>
			<h2 id="secret-title" className="font-semibold">
				{t("webhooks.secret_title")}
			</h2>
			<p className="mt-1 text-sm">{t("webhooks.secret_body")}</p>
			<div className="mt-3 flex flex-wrap items-center gap-2">
				<code className="min-w-0 flex-1 break-all rounded-md bg-card px-3 py-2 font-mono text-sm text-ink">
					{secret}
				</code>
				<button
					type="button"
					onClick={copy}
					aria-live="polite"
					className="rounded-md bg-action-600 px-3 py-2 text-sm font-medium text-on-action hover:bg-action-700"
				>
					{copied ? t("webhooks.copied") : t("webhooks.copy")}
				</button>
				<button
					type="button"
					onClick={() => setOpen(false)}
					className="rounded-md px-3 py-2 text-sm font-medium underline-offset-2 hover:underline"
				>
					{t("webhooks.dismiss")}
				</button>
			</div>
		</section>
	);
}

export default function Webhooks({ hasShop, endpoints, events, maxCount, secret }: Props): JSX.Element {
	const { t, locale } = useT();
	const form = useForm<{ url: string; events: WebhookEvent[] }>({ url: "", events: ["order.paid"] });
	const date = new Intl.DateTimeFormat(locale, { dateStyle: "medium" });

	const toggle = (event: WebhookEvent, on: boolean) => {
		form.setData("events", on ? [...form.data.events, event] : form.data.events.filter((e) => e !== event));
	};

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/seller/webhooks", { preserveScroll: true, onSuccess: () => form.reset("url") });
	};

	const remove = (id: number) => {
		if (window.confirm(t("webhooks.delete_confirm"))) {
			router.delete(`/seller/webhooks/${id}`, { preserveScroll: true });
		}
	};

	return (
		<>
			<Head title={t("webhooks.title")} />
			<h1 className="text-2xl font-semibold text-ink">{t("webhooks.title")}</h1>
			<p className="mt-1 max-w-prose text-muted">{t("webhooks.intro")}</p>

			{!hasShop ? (
				<p className="mt-6 rounded-lg bg-card px-5 py-4 text-ink shadow-sm">{t("webhooks.no_shop")}</p>
			) : (
				<div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
					<div>
						{secret && <SecretPanel secret={secret} />}
						<form onSubmit={submit} className="space-y-4 rounded-lg bg-card px-5 py-5 shadow-sm">
							<h2 className="font-semibold text-ink">{t("webhooks.new")}</h2>
							<Field
								label={t("webhooks.url")}
								type="url"
								inputMode="url"
								required
								maxLength={2048}
								placeholder="https://"
								value={form.data.url}
								onChange={(e) => form.setData("url", e.target.value)}
								error={form.errors.url}
							/>
							<p className="-mt-2 text-xs text-muted">{t("webhooks.url_hint")}</p>
							<fieldset>
								<legend className="text-sm font-medium text-ink">{t("webhooks.events")}</legend>
								<div className="mt-2 space-y-2">
									{events.map((event) => (
										<label key={event} className="flex items-center gap-2 text-sm text-ink">
											<input
												type="checkbox"
												checked={form.data.events.includes(event)}
												onChange={(e) => toggle(event, e.target.checked)}
												className="size-4 accent-action-600"
											/>
											{t(eventLabel[event])}
											<code className="text-xs text-muted">{event}</code>
										</label>
									))}
								</div>
								{form.errors.events && (
									<p className="mt-1 text-sm text-danger-600">{form.errors.events}</p>
								)}
							</fieldset>
							<div className="flex flex-wrap items-center gap-3">
								<button
									type="submit"
									disabled={form.processing || endpoints.length >= maxCount}
									className="rounded-md bg-action-600 px-4 py-2 font-medium text-on-action hover:bg-action-700 disabled:opacity-50"
								>
									{form.processing ? t("webhooks.adding") : t("webhooks.add")}
								</button>
								<span className="text-xs text-muted">
									{t("webhooks.limit_note", { max: maxCount })}
								</span>
							</div>
						</form>
					</div>

					<section aria-labelledby="endpoints-title" className="rounded-lg bg-card px-5 py-5 shadow-sm">
						<h2 id="endpoints-title" className="font-semibold text-ink">
							{t("webhooks.list_title")}
						</h2>
						{endpoints.length === 0 ? (
							<p className="mt-3 text-sm text-muted">{t("webhooks.empty")}</p>
						) : (
							<ul className="mt-3 divide-y divide-line">
								{endpoints.map((ep) => (
									<li key={ep.id} className="flex flex-wrap items-start gap-3 py-3">
										<div className="min-w-0 flex-1">
											<p className="break-all font-mono text-sm text-ink">{ep.url}</p>
											<p className="mt-1 text-xs text-muted">
												{ep.events.map((e) => t(eventLabel[e])).join(", ")}
											</p>
											<p className="mt-1 text-xs text-muted">
												<span className={ep.active ? "text-success-600" : "text-danger-600"}>
													{ep.active ? t("webhooks.active") : t("webhooks.inactive")}
												</span>{" "}
												<time dateTime={ep.created_at}>
													{date.format(new Date(ep.created_at))}
												</time>
											</p>
										</div>
										<div className="flex gap-2">
											<Link
												href={`/seller/webhooks/${ep.id}`}
												className="rounded-md px-3 py-1.5 text-sm font-medium text-action-600 hover:bg-action-500/10"
											>
												{t("webhooks.deliveries")}
											</Link>
											<button
												type="button"
												onClick={() => remove(ep.id)}
												className="rounded-md px-3 py-1.5 text-sm font-medium text-danger-600 hover:bg-danger-600/10"
											>
												{t("webhooks.delete")}
											</button>
										</div>
									</li>
								))}
							</ul>
						)}
					</section>
				</div>
			)}
		</>
	);
}
