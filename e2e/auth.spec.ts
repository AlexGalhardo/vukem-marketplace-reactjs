import { expect, test } from "@playwright/test";

const password = "Correct-Horse-Battery-9";

test("register, log out and log back in", async ({ page }) => {
	const email = `buyer-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;

	await page.goto("/register");
	await page.getByLabel("Name").fill("Ana Buyer");
	await page.getByLabel("Email").fill(email);
	await page.getByLabel(/^Password/).fill(password);
	await page.getByLabel("Confirm password").fill(password);
	await page.getByRole("button", { name: /register|create/i }).click();

	await expect(page).toHaveURL(/\/email\/verify$/);
	await expect(page.getByRole("link", { name: "Ana Buyer" })).toBeVisible();

	await page.getByRole("button", { name: "Log out" }).click();
	await expect(page.getByRole("link", { name: "Log in" })).toBeVisible();

	await page.goto("/login");
	await page.getByLabel("Email").fill(email);
	await page.getByLabel("Password").fill(password);
	await page.getByRole("button", { name: /log in/i }).click();
	await expect(page).toHaveURL(/\/dashboard$/);
});

test("wrong password shows a generic error", async ({ page }) => {
	await page.goto("/login");
	await page.getByLabel("Email").fill(`nobody-${Date.now()}@example.com`);
	await page.getByLabel("Password").fill("not-the-password");
	await page.getByRole("button", { name: /log in/i }).click();
	await expect(page).toHaveURL(/\/login$/);
	await expect(page.getByText(/credentials|incorrect|invalid/i)).toBeVisible();
});
