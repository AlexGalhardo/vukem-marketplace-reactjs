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
	// Units in the signed-in user's cart (header badge).
	cartCount?: number;
	[key: string]: unknown;
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
