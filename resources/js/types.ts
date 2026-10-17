// Shapes of the props the Go controllers send.

export interface User {
	id: number;
	name: string;
	email: string;
	email_verified_at: string | null;
	role: "user" | "admin";
}

export interface SharedProps {
	appName: string;
	locale: string;
	locales: string[];
	errors: Record<string, string>;
	// Present once the Auth phase ships.
	auth?: { user: User | null };
	// Units in the signed-in user's cart (header badge).
	cartCount?: number;
	[key: string]: unknown;
}

// Account/Security page props (controllers/account.ShowSecurity).
export interface SecurityProps {
	twoFactorEnabled: boolean;
	recoveryCodesLeft: number;
}

// Catalog (app/catalog). Money is integer cents (BRL).

export interface CategoryRef {
	slug: string;
	name: string;
}

export interface ProductCard {
	id: number;
	slug: string;
	title: string;
	price_cents: number;
	stock: number;
	shop_slug: string;
	shop_name: string;
	ship_from_uf: string;
	image_url: string;
	created_at: string;
}

export interface Page<T> {
	data: T[];
	total: number;
	page: number;
	per_page: number;
	last_page: number;
}

export interface ImageView {
	id: number;
	processing: boolean;
	url_320: string;
	url_640: string;
	url_1280: string;
	width: number;
	height: number;
	alt: string;
}

export interface ShopView {
	id: number;
	slug: string;
	name: string;
	description: string;
	logo_url: string;
	ship_from_uf: string;
	status: string;
	stripe_ready: boolean;
	since: string;
}

export interface ProductDetail {
	id: number;
	slug: string;
	title: string;
	description: string;
	price_cents: number;
	stock: number;
	weight_grams: number;
	rating_avg_x100: number;
	rating_count: number;
	category: CategoryRef | null;
	images: ImageView[];
	shop: ShopView;
}

export type ProductStatus = "draft" | "published" | "archived" | "suspended";

export interface SellerProduct {
	id: number;
	slug: string;
	title: string;
	description: string;
	category: string;
	price_cents: number;
	stock: number;
	weight_grams: number;
	status: ProductStatus;
	images: ImageView[];
	updated_at: string;
}

export interface SearchQuery {
	q: string;
	category: string;
	shop: string;
	min: number;
	max: number;
	uf: string;
	sort: "relevance" | "price_asc" | "price_desc" | "newest";
	page: number;
	per_page: number;
}

// Commerce (cart, checkout, payouts). Money is integer cents (BRL).

export type CartIssue = "" | "unpublished" | "shop_suspended" | "own_shop" | "out_of_stock" | "insufficient_stock";

export interface CartLine {
	id: number;
	productId: number;
	slug: string;
	title: string;
	priceCents: number;
	quantity: number;
	stock: number;
	weightGrams: number;
	shopId: number;
	shopName: string;
	shopSlug: string;
	lineTotalCents: number;
	issue: CartIssue;
}

export interface CartView {
	lines: CartLine[];
	subtotalCents: number;
	itemCount: number;
	hasIssues: boolean;
}

export interface Address {
	id: number;
	recipient: string;
	cep: string;
	street: string;
	number: string;
	complement: string;
	district: string;
	city: string;
	uf: string;
	phone: string;
	isDefault: boolean;
}

export interface CheckoutGroup {
	shopId: number;
	shopName: string;
	lines: CartLine[];
	weightGrams: number;
	subtotalCents: number;
	shippingCents: number;
	days: number;
	acceptsCard: boolean;
}

export interface CheckoutQuote {
	address: Address;
	groups: CheckoutGroup[];
	subtotalCents: number;
	shippingCents: number;
	totalCents: number;
	acceptsCard: boolean;
}

export type OrderStatus = "pending" | "paid" | "partially_refunded" | "refunded" | "canceled" | "expired";

export interface OrderSummary {
	publicId: string;
	status: OrderStatus;
	paymentMethod: "card" | "pix";
	subtotalCents: number;
	shippingCents: number;
	totalCents: number;
	pixExpiresAt?: string;
}

export interface PayoutRow {
	id: number;
	shopId: number;
	shopName: string;
	amountCents: number;
	status: "requested" | "paid" | "rejected";
	endToEndId: string;
	rejectedReason: string;
	createdAt: string;
	paidAt?: string;
}

export interface BalanceSummary {
	balanceCents: number;
	requestedCents: number;
	availableCents: number;
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
