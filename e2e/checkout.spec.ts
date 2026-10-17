import { expect, test } from "@playwright/test";

// Checkout flows on the integrated stack. They need seeded data the
// catalog and auth streams provide (a verified buyer, a published product
// from another seller) and, for PIX, AbacatePay dev-mode keys. Until the
// seeders land (A.1b) the specs read their fixtures from the environment
// and skip without them.
const buyerEmail = process.env.E2E_BUYER_EMAIL;
const buyerPassword = process.env.E2E_BUYER_PASSWORD;
const productSlug = process.env.E2E_PRODUCT_SLUG;
const pixEnabled = Boolean(process.env.ABACATEPAY_API_KEY);

test.describe("checkout", () => {
	test.skip(!buyerEmail || !buyerPassword || !productSlug, "E2E_BUYER_* and E2E_PRODUCT_SLUG are not set");

	test.beforeEach(async ({ page }) => {
		await page.goto("/login");
		await page.getByLabel("Email").fill(buyerEmail ?? "");
		await page.getByLabel("Password").fill(buyerPassword ?? "");
		await page.getByRole("button", { name: /log in/i }).click();
		await expect(page).toHaveURL(/\/dashboard$/);
	});

	test("cart re-prices and ignores client prices", async ({ page, request }) => {
		await page.goto(`/p/${productSlug}`);
		await page.getByRole("button", { name: "Add to cart" }).click();
		await page.goto("/cart");
		await expect(page.getByRole("heading", { name: "Your cart" })).toBeVisible();

		// A tampered request: the server keeps its own price.
		const res = await request.post("/cart/items", {
			data: { slug: productSlug, quantity: 1, priceCents: 1 },
			headers: { "X-Inertia": "true" },
		});
		expect(res.status()).toBeLessThan(500);
		await page.reload();
		await expect(page.getByText("R$ 0,01")).toHaveCount(0);
	});

	test("freight box quotes a CEP", async ({ page }) => {
		await page.goto(`/p/${productSlug}`);
		await page.getByLabel("CEP").fill("01310-100");
		await page.getByRole("button", { name: "Calculate" }).click();
		await expect(page.getByText(/to SP, about \d+ business days/)).toBeVisible();
	});

	test("PIX checkout shows a QR code and pays in dev mode", async ({ page }) => {
		test.skip(!pixEnabled, "ABACATEPAY_API_KEY is not set");

		await page.goto(`/p/${productSlug}`);
		await page.getByRole("button", { name: "Add to cart" }).click();
		await page.goto("/checkout");
		await expect(page).toHaveURL(/\/checkout\/address$/);
		await page.getByRole("button", { name: "Deliver here" }).click();
		await expect(page).toHaveURL(/\/checkout\/payment$/);
		await page.getByLabel("PIX").check();
		await page.getByRole("button", { name: /^Pay / }).click();

		await expect(page).toHaveURL(/\/checkout\/pix\/[0-9a-f-]{36}$/);
		await expect(page.getByRole("img", { name: "PIX QR code" })).toBeVisible();
		await expect(page.getByLabel("PIX copy-and-paste code")).not.toBeEmpty();

		await page.getByRole("button", { name: "Simulate payment (dev mode)" }).click();
		// The webhook reaches the app only on a public HTTPS URL (staging).
		await expect(page.getByText("Payment received. Thank you!")).toBeVisible({ timeout: 30_000 });
	});
});

test("webhooks reject unsigned requests", async ({ request }) => {
	const stripe = await request.post("/webhooks/stripe", {
		data: { id: "evt_x", type: "checkout.session.completed" },
	});
	expect([400, 503]).toContain(stripe.status());

	const pix = await request.post("/webhooks/abacatepay?webhookSecret=wrong", { data: { id: "evt_x" } });
	expect([401, 503]).toContain(pix.status());
});

test("cart and checkout need a session", async ({ page }) => {
	await page.goto("/cart");
	await expect(page).toHaveURL(/\/login$/);
	await page.goto("/checkout/address");
	await expect(page).toHaveURL(/\/login$/);
});
