import { expect, type Page, test } from "@playwright/test";

import { testPassword as password, register as signUp, totp, uniqueEmail } from "./support";

// register signs up; the landing page depends on the email_verification
// flag (off by default: the dashboard), which e2e/flags.spec.ts toggles.
async function register(page: Page, name: string, email: string): Promise<void> {
	await signUp(page, name, email, password);
}

async function logIn(page: Page, email: string): Promise<void> {
	await page.goto("/login");
	await page.getByLabel("Email").fill(email);
	await page.getByLabel("Password").fill(password);
	await page.getByRole("button", { name: /log in/i }).click();
}

test("register, log out and log back in", async ({ page }) => {
	const email = uniqueEmail("buyer");

	await register(page, "Ana Buyer", email);
	await expect(page.getByRole("link", { name: "Ana Buyer" })).toBeVisible();

	await page.getByRole("button", { name: "Log out" }).click();
	await expect(page.getByRole("link", { name: "Log in" })).toBeVisible();

	await logIn(page, email);
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

test("account pages: profile, password change, 2FA and the login challenge", async ({ page }) => {
	const email = uniqueEmail("account");
	await register(page, "Bea Account", email);

	await page.getByRole("link", { name: "My account" }).click();
	await expect(page).toHaveURL(/\/account$/);
	await page.getByLabel("Name").fill("Bea Renamed");
	await page.getByRole("button", { name: "Save" }).click();
	await expect(page.getByRole("status")).toContainText("profile was saved");
	await expect(page.getByRole("link", { name: "Bea Renamed" })).toBeVisible();

	await page.getByRole("link", { name: /^Security/ }).click();
	await expect(page).toHaveURL(/\/account\/security$/);
	await expect(page.getByRole("heading", { name: "Other sessions" })).toBeVisible();

	// Enable 2FA with the setup key, like an authenticator app would.
	await page.getByRole("link", { name: "Manage two-factor authentication" }).click();
	await page.getByRole("button", { name: "Enable two-factor authentication" }).click();
	await expect(page.getByRole("img", { name: "Two-factor QR code" })).toBeVisible();
	const secret = (await page.locator("code").textContent())?.trim() ?? "";
	expect(secret).toMatch(/^[A-Z2-7]+=*$/);
	await page.getByLabel("Code").fill(totp(secret, Date.now()));
	await page.getByRole("button", { name: "Confirm" }).click();
	await expect(page.getByText("Store these recovery codes somewhere safe.")).toBeVisible();

	await page.getByRole("button", { name: "Log out" }).click();
	// Wait for the logout to land before visiting /login (a guest-only page).
	await expect(page.getByRole("link", { name: "Log in" })).toBeVisible();

	// The password alone stops at the challenge; the next time step logs in
	// (each step works once, and confirming used the current one).
	await logIn(page, email);
	await expect(page).toHaveURL(/\/two-factor-challenge$/);
	await page.goto("/dashboard");
	await expect(page).toHaveURL(/\/login$/);

	await logIn(page, email);
	await expect(page).toHaveURL(/\/two-factor-challenge$/);
	await page.getByLabel("Code").fill(totp(secret, Date.now() + 30_000));
	await page.getByRole("button", { name: "Log in" }).click();
	await expect(page).toHaveURL(/\/dashboard$/);
});

test("auth pages follow the PT-BR locale", async ({ page }) => {
	await page.goto("/login");
	await page.getByTestId("locale-menu").click();
	await page.getByRole("menuitemradio", { name: "Português (Brasil)" }).click();
	await expect(page.getByRole("heading", { name: "Entrar" })).toBeVisible();

	await page.getByLabel("E-mail").fill(`ninguem-${Date.now()}@example.com`);
	await page.getByLabel("Senha").fill("nao-e-a-senha");
	await page.getByRole("button", { name: "Entrar" }).click();
	await expect(page.getByText("Essas credenciais não correspondem aos nossos registros.")).toBeVisible();
});
