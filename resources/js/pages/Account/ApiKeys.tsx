import { Head, Link, router, useForm } from "@inertiajs/react";
import { type FormEvent, useState } from "react";

import Field from "../../components/Field";
import { type MessageKey, useT } from "../../lib/i18n";

interface ApiKey {
	id: number;
	name: string;
	prefix: string;
	scopes: string[];
	last_used_at: string | null;
	expires_at: string | null;
	revoked_at: string | null;
	created_at: string;
	active: boolean;
}

interface Props {
	keys: ApiKey[];
	scopes: string[];
	expiryDays: number[];
	maxKeys: number;
	created: { name: string; token: string } | null;
	errors?: Record<string, string>;
}

const button = "rounded-md bg-action-500 px-4 py-2 font-medium text-on-action hover:bg-action-600 disabled:opacity-50";

export default function ApiKeys({ keys, scopes, expiryDays, maxKeys, created }: Props) {
	const { t, locale } = useT();
	const form = useForm<{ name: string; scopes: string[]; expires_in_days: number }>({
		name: "",
		scopes: ["catalog:read"],
		expires_in_days: expiryDays.includes(90) ? 90 : 0,
	});
	const active = keys.filter((k) => k.active).length;
	const date = (iso: string) => new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(new Date(iso));

	const submit = (e: FormEvent) => {
		e.preventDefault();
		form.post("/account/api-keys", { preserveScroll: true, onSuccess: () => form.reset("name") });
	};

	const toggleScope = (scope: string, on: boolean) => {
		form.setData("scopes", on ? [...form.data.scopes, scope] : form.data.scopes.filter((s) => s !== scope));
	};

	const revoke = (key: ApiKey) => {
		if (window.confirm(t("api.keys.revoke_confirm", { name: key.name }))) {
			router.delete(`/account/api-keys/${key.id}`, { preserveScroll: true });
		}
	};

	return (
		<>
			<Head title={t("api.keys.title")} />
			<div className="mx-auto max-w-3xl space-y-8">
				<header className="space-y-2">
					<h1 className="text-2xl font-semibold text-ink">{t("api.keys.title")}</h1>
					<p className="max-w-prose text-muted">{t("api.keys.intro")}</p>
					<a href="/docs/api" className="text-action-600 hover:underline">
						{t("api.keys.docs")}
					</a>
				</header>

				{created && <CreatedKey name={created.name} token={created.token} />}

				<section aria-labelledby="create-key" className="rounded-md bg-card p-6 shadow-card">
					<h2 id="create-key" className="mb-4 text-lg font-semibold text-ink">
						{t("api.keys.create_title")}
					</h2>
					<form onSubmit={submit} className="space-y-5">
						<div>
							<Field
								label={t("api.keys.name")}
								required
								maxLength={60}
								value={form.data.name}
								onChange={(e) => form.setData("name", e.target.value)}
								error={form.errors.name}
							/>
							<p className="mt-1 text-sm text-muted">{t("api.keys.name_hint")}</p>
						</div>

						<fieldset>
							<legend className="mb-2 text-sm font-medium text-ink">{t("api.keys.scopes")}</legend>
							<div className="grid gap-2 sm:grid-cols-2">
								{scopes.map((scope) => (
									<label key={scope} className="flex items-center gap-2 text-sm text-ink">
										<input
											type="checkbox"
											checked={form.data.scopes.includes(scope)}
											onChange={(e) => toggleScope(scope, e.target.checked)}
											className="size-4 accent-action-500"
										/>
										<span>{t(`api.scope.${scope}` as MessageKey)}</span>
										<code className="text-xs text-muted">{scope}</code>
									</label>
								))}
							</div>
							{form.errors.scopes && <p className="mt-1 text-sm text-danger-600">{form.errors.scopes}</p>}
						</fieldset>

						<label className="flex flex-col gap-1 text-sm font-medium text-ink">
							{t("api.keys.expires")}
							<select
								value={form.data.expires_in_days}
								onChange={(e) => form.setData("expires_in_days", Number(e.target.value))}
								className="h-10 w-48 rounded-md border border-line bg-card px-3 text-sm text-ink"
							>
								{expiryDays.map((d) => (
									<option key={d} value={d}>
										{d === 0
											? t("api.keys.expires_never")
											: t("api.keys.expires_days", { days: d })}
									</option>
								))}
							</select>
							{form.errors.expires_in_days && (
								<span className="text-sm font-normal text-danger-600">
									{form.errors.expires_in_days}
								</span>
							)}
						</label>

						<button type="submit" disabled={form.processing || active >= maxKeys} className={button}>
							{t("api.keys.create")}
						</button>
					</form>
				</section>

				<section aria-labelledby="key-list" className="space-y-3">
					<div className="flex items-baseline justify-between">
						<h2 id="key-list" className="text-lg font-semibold text-ink">
							{t("api.keys.list_title")}
						</h2>
						<p className="text-sm text-muted">{t("api.keys.limit", { count: active, max: maxKeys })}</p>
					</div>
					{keys.length === 0 ? (
						<p className="text-muted">{t("api.keys.empty")}</p>
					) : (
						<ul className="divide-y divide-line rounded-md bg-card shadow-card">
							{keys.map((key) => (
								<li key={key.id} className="flex flex-wrap items-start justify-between gap-3 p-4">
									<div className="min-w-0 space-y-1">
										<p className="font-medium text-ink">
											{key.name} <Status k={key} />
										</p>
										<p className="font-mono text-sm break-all text-muted">{key.prefix}_…</p>
										<p className="text-sm text-muted">{key.scopes.join(", ")}</p>
										<p className="text-sm text-muted">
											{t("api.keys.last_used")}:{" "}
											{key.last_used_at ? date(key.last_used_at) : t("api.keys.never_used")}
											{key.expires_at &&
												` / ${t("api.keys.expires_at")}: ${date(key.expires_at)}`}
										</p>
									</div>
									{key.active && (
										<button
											type="button"
											onClick={() => revoke(key)}
											className="rounded-md border border-danger-600 px-3 py-1.5 text-sm text-danger-600 hover:bg-danger-600 hover:text-on-status"
										>
											{t("api.keys.revoke")}
										</button>
									)}
								</li>
							))}
						</ul>
					)}
				</section>

				<p className="text-sm">
					<Link href="/dashboard" className="text-action-600 hover:underline">
						{t("api.keys.back")}
					</Link>
				</p>
			</div>
		</>
	);
}

function Status({ k }: { k: ApiKey }) {
	const { t } = useT();
	const [label, tone] = k.active
		? [t("api.keys.status_active"), "bg-success-600/15 text-success-600"]
		: k.revoked_at
			? [t("api.keys.status_revoked"), "bg-danger-600/15 text-danger-600"]
			: [t("api.keys.status_expired"), "bg-line text-muted"];

	return <span className={`ml-2 rounded px-2 py-0.5 text-xs font-medium ${tone}`}>{label}</span>;
}

// CreatedKey is the one moment the secret exists outside the user's hands:
// shown once, on the brand yellow, with a copy button.
function CreatedKey({ name, token }: { name: string; token: string }) {
	const { t } = useT();
	const [copied, setCopied] = useState(false);

	const copy = async () => {
		try {
			await navigator.clipboard.writeText(token);
			setCopied(true);
		} catch {
			setCopied(false);
		}
	};

	return (
		<section
			role="status"
			className="space-y-3 rounded-md border-2 border-brand-500 bg-brand-50 p-6 text-brand-900"
		>
			<h2 className="text-lg font-semibold">{t("api.keys.created_title")}</h2>
			<p className="max-w-prose text-sm">{t("api.keys.created_body", { name })}</p>
			<div className="flex flex-wrap items-center gap-3">
				<code className="rounded bg-white px-3 py-2 font-mono text-sm break-all text-ink">{token}</code>
				<button type="button" onClick={copy} className={button}>
					{copied ? t("api.keys.copied") : t("api.keys.copy")}
				</button>
			</div>
		</section>
	);
}
