import type { JSX } from "react";

import { type MessageKey, useT } from "../../lib/i18n";
import type { ProductStatus } from "../../types";

const styles: Record<ProductStatus, { className: string; label: MessageKey }> = {
	draft: { className: "bg-line text-ink", label: "seller.status.draft" },
	published: { className: "bg-success-600/15 text-success-600", label: "seller.status.published" },
	archived: { className: "bg-line text-muted", label: "seller.status.archived" },
	suspended: { className: "bg-danger-600/15 text-danger-600", label: "seller.status.suspended" },
};

// StatusBadge names a product's status.
export default function StatusBadge({ status }: { status: ProductStatus }): JSX.Element {
	const { t } = useT();
	const s = styles[status];

	return (
		<span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${s.className}`}>{t(s.label)}</span>
	);
}
