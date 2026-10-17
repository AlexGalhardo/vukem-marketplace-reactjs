import AxeBuilder from "@axe-core/playwright";
import { type BrowserContext, type Cookie, expect, type Page, test } from "@playwright/test";

import { appCommand, canRunAppCommands, createAdmin, demoPassword, sessionCookies, uniqueEmail } from "./support";

// Accessibility: axe-core scans every page and UI state in light and dark
// themes, EN and PT-BR, on desktop and mobile (the two Playwright
// projects), logged out and as buyer, seller and admin. Serious and
// critical WCAG 2.x A/AA violations fail the test.

// Each test scans many pages; axe takes about a second per page.
test.describe.configure({ timeout: 120_000 });

const combos = [
	{ theme: "light", locale: "en" },
	{ theme: "dark", locale: "en" },
	{ theme: "light", locale: "pt-BR" },
	{ theme: "dark", locale: "pt-BR" },
] as const;

type Combo = (typeof combos)[number];

const product = "/p/furadeira-de-impacto-650-w";

async function prepare(context: BrowserContext, combo: Combo, cookies: Cookie[] = []): Promise<void> {
	const baseURL = test.info().project.use.baseURL ?? "http://localhost:3200";
	await context.addInitScript((theme) => {
		try {
			localStorage.setItem("theme", theme);
		} catch {
			// Storage blocked: the page falls back to prefers-color-scheme.
		}
	}, combo.theme);
	await context.addCookies([{ name: "lang", value: combo.locale, url: baseURL }, ...cookies]);
}

async function scan(page: Page, label: string): Promise<void> {
	await page.waitForLoadState("networkidle");
	const results = await new AxeBuilder({ page })
		.withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
		.analyze();
	const blocking = results.violations
		.filter((v) => v.impact === "serious" || v.impact === "critical")
		.map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
	expect(blocking, `${label} at ${page.url()}`).toEqual([]);
}

async function scanAll(page: Page, combo: Combo, paths: string[]): Promise<void> {
	for (const path of paths) {
		await page.goto(path);
		await scan(page, `${combo.theme}/${combo.locale} ${path}`);
	}
}

for (const combo of combos) {
	test.describe(`a11y ${combo.theme} ${combo.locale}`, () => {
		test("public pages and the open locale menu", async ({ page, context }) => {
			await prepare(context, combo);
			await scanAll(page, combo, [
				"/",
				"/search?q=furadeira",
				"/search?q=zzzz-nothing",
				"/c/tools",
				product,
				"/s/loja-demo-vukem",
				"/login",
				"/register",
				"/forgot-password",
				"/contact",
				"/p/does-not-exist",
			]);

			await page.goto("/");
			await page.getByTestId("locale-menu").click();
			await expect(page.getByRole("menu")).toBeVisible();
			await scan(page, `${combo.theme}/${combo.locale} locale menu open`);
		});

		test("form errors", async ({ page, context }) => {
			await prepare(context, combo);
			await page.goto("/login");
			await page.getByRole("textbox").first().fill(uniqueEmail("nobody"));
			await page.locator('input[type="password"]').fill("not-the-password");
			await page.locator('main form button[type="submit"]').click();
			await expect(page.locator('[role="alert"], [data-invalid]').first()).toBeVisible();
			await scan(page, `${combo.theme}/${combo.locale} login error`);

			await page.goto("/register");
			await page.locator('main form button[type="submit"]').click();
			await scan(page, `${combo.theme}/${combo.locale} register errors`);
		});
	});
}

test.describe("a11y buyer", () => {
	test.describe.configure({ mode: "serial" });

	let cookies: Cookie[] = [];

	test.beforeAll(async ({ browser }) => {
		cookies = await sessionCookies(await browser.newContext(), "buyer@example.com", demoPassword);
	});

	for (const combo of combos) {
		test(`account, cart and checkout ${combo.theme} ${combo.locale}`, async ({ page, context }) => {
			await prepare(context, combo, cookies);
			await scanAll(page, combo, [
				"/dashboard",
				"/account",
				"/account/security",
				"/account/addresses",
				"/account/api-keys",
				"/user/two-factor",
			]);

			await page.goto(product);
			await page.locator('main form button[type="submit"]').first().click();
			await scanAll(page, combo, ["/cart", "/checkout/address"]);

			await page.goto("/checkout/payment");
			await scan(page, `${combo.theme}/${combo.locale} checkout payment (or its redirect)`);
		});

		// A paid order (dev:paid-order) makes the demo drill reviewable: the
		// write link, the form, its errors and the list with vote buttons.
		test(`review form and list ${combo.theme} ${combo.locale}`, async ({ page, context }) => {
			test.skip(!canRunAppCommands, "needs the compose app container or E2E_APP_BIN for dev:paid-order");
			await prepare(context, combo, cookies);
			appCommand("dev:paid-order", "buyer@example.com", product.replace("/p/", ""));

			await page.goto(product);
			await scan(page, `${combo.theme}/${combo.locale} product with write link`);
			await page.locator('a[href^="/order-items/"]').first().click();
			await expect(page).toHaveURL(/\/order-items\/\d+\/review$/);
			await scan(page, `${combo.theme}/${combo.locale} review form`);

			await page.locator('main form button[type="submit"]').click();
			await expect(page.getByRole("alert").first()).toBeVisible();
			await scan(page, `${combo.theme}/${combo.locale} review form errors`);

			await page.locator('main form input[name="rating"][value="5"]').check({ force: true });
			await page.locator('main form button[type="submit"]').click();
			await expect(page).toHaveURL(new RegExp(`${product}$`));
			await page.getByLabel(/^(Sort by|Ordenar por)$/).selectOption("helpful");
			await expect(page).toHaveURL(/rsort=helpful/);
			await scan(page, `${combo.theme}/${combo.locale} product reviews sorted`);
		});
	}
});

test.describe("a11y seller", () => {
	test.describe.configure({ mode: "serial" });

	let cookies: Cookie[] = [];

	test.beforeAll(async ({ browser }) => {
		cookies = await sessionCookies(await browser.newContext(), "seller@example.com", demoPassword);
	});

	for (const combo of combos) {
		test(`seller dashboard ${combo.theme} ${combo.locale}`, async ({ page, context }) => {
			await prepare(context, combo, cookies);
			await scanAll(page, combo, [
				"/seller",
				"/seller/products",
				"/seller/products/new",
				"/seller/settings",
				"/seller/payouts",
				"/seller/webhooks",
				// The demo seller owns the drill: reply forms under its reviews.
				product,
			]);

			await page.goto("/seller/products");
			const edit = page.locator('a[href^="/seller/products/"][href$="/edit"]').first();
			await edit.click();
			await scan(page, `${combo.theme}/${combo.locale} product edit`);
		});
	}
});

test.describe("a11y admin", () => {
	test.describe.configure({ mode: "serial" });
	test.skip(!canRunAppCommands, "needs the compose app container or E2E_APP_BIN for users:make-admin");

	let cookies: Cookie[] = [];

	// A fresh admin: register, verify through Mailpit, enable 2FA, promote
	// with the CLI (the only way to create admins).
	test.beforeAll(async ({ browser }) => {
		const context = await browser.newContext();
		await createAdmin(await context.newPage(), "Ada Admin", uniqueEmail("a11y-admin"));
		cookies = (await context.storageState()).cookies;
		await context.close();
	});

	for (const combo of combos) {
		test(`admin pages ${combo.theme} ${combo.locale}`, async ({ page, context }) => {
			await prepare(context, combo, cookies);
			await scanAll(page, combo, [
				"/admin",
				"/admin/payouts",
				"/admin/flags",
				"/admin/reviews",
				"/admin/reviews?filter=hidden",
			]);
		});
	}
});
