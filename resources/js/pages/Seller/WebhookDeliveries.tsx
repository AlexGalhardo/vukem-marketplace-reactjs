import { Head, Link } from "@inertiajs/react";
import type { JSX } from "react";

import { type MessageKey, useT } from "../../lib/i18n";
import { eventLabel } from "../../lib/webhooks";
import type { WebhookDelivery, WebhookEndpoint } from "../../types";

interface Props {
	endpoint: WebhookEndpoint;
	deliveries: WebhookDelivery[];
}

const statusLabel: Record<WebhookDelivery["status"], MessageKey> = {
	pending: "webhooks.status_pending",
	delivered: "webhooks.status_delivered",
	failed: "webhooks.status_failed",
};

const statusTone: Record<WebhookDelivery["status"], string> = {
	pending: "text-muted",
	delivered: "text-success-600",
	failed: "text-danger-600",
};

export default function WebhookDeliveries({ endpoint, deliveries }: Props): JSX.Element {
	const { t, locale } = useT();
	const when = new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "medium" });

	return (
		<>
			<Head title={t("webhooks.deliveries")} />
			<Link href="/seller/webhooks" className="text-sm font-medium text-action-600 hover:underline">
				{t("webhooks.back")}
			</Link>
			<h1 className="mt-2 text-2xl font-semibold text-ink">{t("webhooks.deliveries")}</h1>
			<p className="mt-1 break-all font-mono text-sm text-muted">{endpoint.url}</p>

			<div className="mt-6 overflow-x-auto rounded-lg bg-card shadow-sm">
				{deliveries.length === 0 ? (
					<p className="px-5 py-4 text-sm text-muted">{t("webhooks.no_deliveries")}</p>
				) : (
					<table className="w-full min-w-[40rem] text-left text-sm">
						<thead className="border-b border-line text-muted">
							<tr>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("webhooks.col_event")}
								</th>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("webhooks.col_status")}
								</th>
								<th scope="col" className="px-4 py-2 text-right font-medium">
									{t("webhooks.col_attempts")}
								</th>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("webhooks.col_response")}
								</th>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("webhooks.col_error")}
								</th>
								<th scope="col" className="px-4 py-2 font-medium">
									{t("webhooks.col_updated")}
								</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-line text-ink">
							{deliveries.map((d) => (
								<tr key={d.event_id}>
									<td className="px-4 py-2">
										{t(eventLabel[d.event])}
										<span className="block font-mono text-xs text-muted">{d.event_id}</span>
									</td>
									<td className={`px-4 py-2 font-medium ${statusTone[d.status]}`}>
										{t(statusLabel[d.status])}
									</td>
									<td className="px-4 py-2 text-right tabular-nums">{d.attempts}</td>
									<td className="px-4 py-2 tabular-nums">{d.response_code ?? "—"}</td>
									<td className="px-4 py-2 font-mono text-xs">{d.last_error ?? "—"}</td>
									<td className="px-4 py-2">
										<time dateTime={d.updated_at}>{when.format(new Date(d.updated_at))}</time>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</div>
		</>
	);
}
