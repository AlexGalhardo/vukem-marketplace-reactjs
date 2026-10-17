import { expect, test } from "@playwright/test";

import { register, testPassword, uniqueEmail } from "./support";

// A signed-in user without a shop never reaches a seller page through the
// navigation, and a typed seller URL lands on /sell instead of an error.
test("users without a shop see no seller links and are sent to /sell", async ({ page }) => {
	await register(page, "Sem Loja", uniqueEmail("noshop"), testPassword);

	const footer = page.locator("footer");
	await expect(footer.locator('a[href="/sell"]')).toBeVisible();
	await expect(footer.locator('a[href="/seller/payouts"]')).toHaveCount(0);
	await expect(footer.locator('a[href="/seller"]')).toHaveCount(0);

	await page.goto("/seller/payouts");
	await expect(page).toHaveURL(/\/sell$/);
	await expect(page.locator("body")).not.toContainText("Record not found");
});

test("a missing product shows the app's error page", async ({ page }) => {
	const res = await page.goto("/p/does-not-exist-e2e");
	expect(res?.status()).toBe(404);
	await expect(page.getByRole("link", { name: /home page|página inicial/i })).toBeVisible();
});
