// Shapes of the props the Go controllers send.

export interface User {
	id: number;
	name: string;
	email: string;
	email_verified_at: string | null;
}

export interface SharedProps {
	appName: string;
	locale: string;
	locales: string[];
	errors: Record<string, string>;
	// Present once the Auth phase ships.
	auth?: { user: User | null };
	[key: string]: unknown;
}

// Seller webhooks (Go: controllers.EndpointView / DeliveryView).
export type WebhookEvent = "order.paid" | "order.refunded" | "product.suspended";

export interface WebhookEndpoint {
	id: number;
	url: string;
	events: WebhookEvent[];
	active: boolean;
	created_at: string;
}

export interface WebhookDelivery {
	event_id: string;
	event: WebhookEvent;
	status: "pending" | "delivered" | "failed";
	attempts: number;
	response_code: number | null;
	last_error: string | null;
	next_attempt_at: string | null;
	updated_at: string;
}
