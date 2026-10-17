import { Switch } from "@base-ui/react/switch";
import { Head, router } from "@inertiajs/react";
import { type JSX, useState } from "react";

import { type MessageKey, useT } from "../../lib/i18n";
import { formatDateTime } from "../../lib/money";
import type { FeatureFlag, FeatureFlagChange } from "../../types";

interface Props {
	flags: FeatureFlag[];
	changes: FeatureFlagChange[];
}

function flagLabel(key: string): MessageKey {
	return `admin.flags.items.${key}.label` as MessageKey;
}

function FlagRow({ flag }: { flag: FeatureFlag }): JSX.Element {
	const { t } = useT();
	const [saving, setSaving] = useState(false);
	const id = `flag-${flag.key}`;

	const toggle = (enabled: boolean) => {
		router.post(
			"/admin/flags",
			{ flag: flag.key, enabled },
			{ preserveScroll: true, onStart: () => setSaving(true), onFinish: () => setSaving(false) },
		);
	};

	return (
		<li className="flex items-start justify-between gap-4 px-4 py-4">
			<div className="min-w-0">
				<p id={`${id}-label`} className="font-medium text-ink">
					{t(flagLabel(flag.key))}
					{flag.locked && (
						<span className="ml-2 rounded bg-surface px-2 py-0.5 text-xs font-normal text-muted">
							{t("admin.flags.locked")}
						</span>
					)}
				</p>
				<p id={`${id}-description`} className="mt-1 max-w-2xl text-sm text-muted">
					{t(`admin.flags.items.${flag.key}.description` as MessageKey)}
				</p>
				<p className="mt-1 text-xs text-muted">
					{t("admin.flags.default", { value: t(flag.default ? "admin.flags.on" : "admin.flags.off") })}
				</p>
			</div>
			<div className="flex shrink-0 items-center gap-2">
				<span className="text-sm text-muted" aria-hidden="true">
					{t(flag.enabled ? "admin.flags.on" : "admin.flags.off")}
				</span>
				<Switch.Root
					checked={flag.enabled}
					disabled={flag.locked || saving}
					onCheckedChange={toggle}
					aria-labelledby={`${id}-label`}
					aria-describedby={`${id}-description`}
					data-testid={id}
					className="relative inline-flex h-6 w-11 items-center rounded-full bg-line transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-action-600 data-checked:bg-action-600 data-disabled:opacity-60 motion-reduce:transition-none"
				>
					<Switch.Thumb className="ml-0.5 size-5 rounded-full bg-white shadow transition-transform data-checked:translate-x-5 motion-reduce:transition-none" />
				</Switch.Root>
			</div>
		</li>
	);
}

export default function AdminFlags({ flags, changes }: Props): JSX.Element {
	const { t, locale } = useT();

	return (
		<>
			<Head title={t("admin.flags.title")} />
			<h1 className="text-2xl font-semibold text-ink">{t("admin.flags.title")}</h1>
			<p className="mt-1 mb-6 max-w-2xl text-muted">{t("admin.flags.intro")}</p>
			<ul className="divide-y divide-line rounded-md bg-card">
				{flags.map((f) => (
					<FlagRow key={f.key} flag={f} />
				))}
			</ul>

			<h2 id="flag-history" className="mt-8 mb-3 text-lg font-semibold text-ink">
				{t("admin.flags.history")}
			</h2>
			{changes.length === 0 ? (
				<p className="text-muted">{t("admin.flags.history_empty")}</p>
			) : (
				<table className="w-full text-left text-sm" aria-labelledby="flag-history">
					<thead className="text-muted">
						<tr>
							<th className="py-2 pr-4 font-normal">{t("admin.flags.col_when")}</th>
							<th className="py-2 pr-4 font-normal">{t("admin.flags.col_flag")}</th>
							<th className="py-2 pr-4 font-normal">{t("admin.flags.col_change")}</th>
							<th className="py-2 font-normal">{t("admin.flags.col_by")}</th>
						</tr>
					</thead>
					<tbody className="divide-y divide-line">
						{changes.map((c) => (
							<tr key={`${c.at}-${c.flag}-${c.new}`}>
								<td className="py-2 pr-4">{formatDateTime(c.at, locale)}</td>
								<td className="py-2 pr-4">{t(flagLabel(c.flag))}</td>
								<td className="py-2 pr-4 whitespace-nowrap">
									{t(c.old ? "admin.flags.on" : "admin.flags.off")} →{" "}
									{t(c.new ? "admin.flags.on" : "admin.flags.off")}
								</td>
								<td className="py-2 break-all">{c.user_email}</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</>
	);
}
