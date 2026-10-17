import type { WebhookEvent } from "../types";
import type { MessageKey } from "./i18n";

export const eventLabel: Record<WebhookEvent, MessageKey> = {
	"order.paid": "webhooks.event_order_paid",
	"order.refunded": "webhooks.event_order_refunded",
	"product.suspended": "webhooks.event_product_suspended",
};
