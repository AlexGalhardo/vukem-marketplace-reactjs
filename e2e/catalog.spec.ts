import { expect, type Page, test } from "@playwright/test";

// Catalog flows against the seeded stack (`vukem db:seed`: categories, the
// demo shop "Loja Demo Vukem" with 12 published products owned by
// seller@example.com). The seller tests log in as that user; set
// E2E_SELLER_PASSWORD when the auth seeder uses another password.
const sellerEmail = "seller@example.com";
const sellerPassword = process.env.E2E_SELLER_PASSWORD ?? "demo-seller-password";

const tinyPNG = Buffer.from(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	"base64",
);

async function trackErrors(page: Page): Promise<string[]> {
	const errors: string[] = [];
	page.on("console", (msg) => {
		if (msg.type() === "error" && !msg.text().includes("favicon")) errors.push(msg.text());
	});
	return errors;
}

test.describe("storefront", () => {
	test("home lists categories and products", async ({ page }) => {
		const errors = await trackErrors(page);
		await page.goto("/");
		await expect(page.getByRole("navigation", { name: "Categories" })).toBeVisible();
		await expect(page.getByRole("heading", { name: "Lowest prices" })).toBeVisible();
		await expect(page.getByRole("heading", { name: "New arrivals" })).toBeVisible();
		await expect(page.locator('a[href^="/p/"]').first()).toBeVisible();
		expect(errors).toEqual([]);
	});

	test("search, filter and open a product, then see it as recently viewed", async ({ page }) => {
		await page.goto("/");
		await page.getByRole("searchbox").fill("Furadeira");
		await page.getByRole("searchbox").press("Enter");
		await expect(page).toHaveURL(/\/search\?q=Furadeira/);
		await expect(page.getByRole("heading", { level: 1 })).toContainText("Furadeira");

		await page.getByLabel("Sort by").selectOption("price_desc");
		await expect(page).toHaveURL(/sort=price_desc/);

		await page.locator('a[href="/p/furadeira-de-impacto-650-w"]').click();
		await expect(page.getByRole("heading", { level: 1 })).toHaveText("Furadeira de Impacto 650 W");
		await expect(page.getByText(/R\$\s?279\.90/)).toBeAttached();
		await expect(page.getByRole("heading", { name: "Shipping" })).toBeVisible();
		await expect(page.getByRole("link", { name: "Loja Demo Vukem" })).toBeVisible();

		await page.goto("/");
		await expect(page.getByRole("heading", { name: "Recently viewed" })).toBeVisible();
	});

	test("category and storefront pages", async ({ page }) => {
		await page.goto("/c/tools");
		await expect(page.getByRole("heading", { level: 1 })).toHaveText("Tools");
		await page.goto("/s/loja-demo-vukem");
		await expect(page.getByRole("heading", { level: 1 })).toHaveText("Loja Demo Vukem");
		await expect(page.locator('a[href^="/p/"]')).not.toHaveCount(0);
	});

	test("unknown or hidden pages are 404", async ({ request }) => {
		expect((await request.get("/p/does-not-exist")).status()).toBe(404);
		expect((await request.get("/s/does-not-exist")).status()).toBe(404);
		expect((await request.get("/c/does-not-exist")).status()).toBe(404);
		expect((await request.get("/media/products/1/..%2f..%2f.env")).status()).toBe(404);
	});

	test("Portuguese prices use BRL formatting", async ({ browser }) => {
		const context = await browser.newContext({ locale: "pt-BR" });
		const page = await context.newPage();
		await page.goto("/p/furadeira-de-impacto-650-w");
		await expect(page.getByText(/R\$\s279,90/)).toBeAttached();
		await expect(page.getByRole("heading", { name: "Frete" })).toBeVisible();
		await context.close();
	});
});

test.describe("seller", () => {
	test.describe.configure({ mode: "serial" });

	test.beforeEach(async ({ page }) => {
		await page.goto("/login");
		await page.getByLabel("Email").fill(sellerEmail);
		await page.getByLabel("Password").fill(sellerPassword);
		await page.getByRole("button", { name: "Log in" }).click();
		await expect(page).not.toHaveURL(/\/login/);
	});

	test("creates, illustrates, publishes and archives a product", async ({ page }) => {
		const title = `E2E Produto ${Date.now()}`;

		await page.goto("/seller/products/new");
		await page.getByLabel("Title").fill(title);
		await page.getByLabel("Category").selectOption("tools");
		await page.getByLabel("Price (R$)").fill("49,90");
		await page.getByLabel("Stock").fill("3");
		await page.getByLabel("Weight (g)").fill("500");
		await page.getByRole("button", { name: "Save draft" }).click();
		await expect(page).toHaveURL(/\/seller\/products\/\d+\/edit/);
		await expect(page.getByText("Draft", { exact: true })).toBeVisible();

		await page.locator('input[type="file"][name="images"]').setInputFiles({
			name: "photo.png",
			mimeType: "image/png",
			buffer: tinyPNG,
		});
		// Thumbnails may run inline (sync queue) on a slow bucket.
		await expect(page.getByRole("heading", { name: /Photos \(1 of 8\)/ })).toBeVisible({ timeout: 30_000 });

		// Abuse: a script disguised as an image is refused.
		await page.locator('input[type="file"][name="images"]').setInputFiles({
			name: "evil.png",
			mimeType: "image/png",
			buffer: Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>'),
		});
		await expect(page.getByRole("alert")).toBeVisible();
		await expect(page.getByRole("heading", { name: /Photos \(1 of 8\)/ })).toBeVisible();

		await page.getByRole("button", { name: "Publish" }).click();
		await expect(page.getByText("Published", { exact: true })).toBeVisible();
		await page.getByRole("link", { name: "View product page" }).click();
		await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);
		const productURL = page.url();

		await page.goBack();
		await page.getByRole("button", { name: "Archive" }).click();
		await expect(page.getByText("Archived", { exact: true })).toBeVisible();
		expect((await page.request.get(productURL)).status()).toBe(404);
	});

	test("rejects invalid prices on the server", async ({ page }) => {
		await page.goto("/seller/products/new");
		await page.getByLabel("Title").fill("Preço inválido");
		await page.getByLabel("Price (R$)").fill("0");
		await page.getByLabel("Stock").fill("1");
		await page.getByLabel("Weight (g)").fill("100");
		await page.getByRole("button", { name: "Save draft" }).click();
		await expect(page).toHaveURL(/\/seller\/products\/new/);
		await expect(page.locator("[data-invalid]").first()).toBeVisible();
	});

	test("another seller's product edit page is 404", async ({ page }) => {
		const res = await page.goto("/seller/products/999999999/edit");
		expect(res?.status()).toBe(404);
	});
});
