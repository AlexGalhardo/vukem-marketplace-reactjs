import { expect, test } from "@playwright/test";

test.describe("smoke", () => {
	test("probes answer JSON", async ({ request }) => {
		const health = await request.get("/health");
		expect(health.status()).toBe(200);
		expect(await health.json()).toMatchObject({ status: "ok" });

		const ready = await request.get("/ready");
		expect(ready.status()).toBe(200);
		expect(await ready.json()).toMatchObject({ status: "ok" });
	});

	test("home renders with security headers and no console errors", async ({ page }) => {
		const errors: string[] = [];
		page.on("console", (msg) => {
			if (msg.type() === "error") errors.push(msg.text());
		});
		const response = await page.goto("/");
		expect(response?.status()).toBe(200);
		expect(response?.headers()["content-security-policy"]).toContain("frame-ancestors 'none'");
		await expect(page.getByRole("heading", { level: 1 })).toHaveText("Everything you need, delivered.");
		expect(errors, "CSP violations or runtime errors").toEqual([]);
	});
});

test("theme toggle switches and persists", async ({ page }) => {
	await page.emulateMedia({ colorScheme: "light" });
	await page.goto("/");
	const html = page.locator("html");
	await expect(html).not.toHaveClass(/dark/);

	await page.getByTestId("theme-toggle").click();
	await expect(html).toHaveClass(/dark/);

	await page.reload();
	await expect(html).toHaveClass(/dark/);
});

test("locale menu switches to Portuguese", async ({ page }) => {
	await page.goto("/");
	await page.getByTestId("locale-menu").click();
	await page.getByRole("menuitemradio", { name: "Português (Brasil)" }).click();

	await expect(page.getByRole("heading", { level: 1 })).toHaveText("Tudo o que você precisa, entregue.");
	await expect(page.locator("html")).toHaveAttribute("lang", "pt-BR");
});
