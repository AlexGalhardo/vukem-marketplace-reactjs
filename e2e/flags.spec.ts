import { expect, type Page, test } from "@playwright/test";

import { canRunAppCommands, createAdmin, register, testPassword, uniqueEmail } from "./support";

// Google sign-in, the name rules of the sign-up form and the admin feature
// flags. The flag test changes global state, so this file runs serially
// and restores the default (email_verification off) when it ends.
test.describe.configure({ mode: "serial" });

const nameError = "Your name must have 4 to 24 characters.";

test.describe("Continue with Google", () => {
	// The button only shows when SOCIAL_GOOGLE_CLIENT_ID is set; set
	// E2E_GOOGLE=1 when the stack under test has it (any value: the test
	// never follows the redirect to Google).
	test.skip(process.env.E2E_GOOGLE !== "1", "the stack has no Google credentials (E2E_GOOGLE=1)");

	for (const path of ["/login", "/register"]) {
		test(`button on ${path} in both locales`, async ({ page, context, baseURL }) => {
			await page.goto(path);
			const google = page.getByRole("link", { name: "Continue with Google" });
			await expect(google).toBeVisible();
			await expect(google).toHaveAttribute("href", "/auth/google/redirect");

			await context.addCookies([{ name: "lang", value: "pt-BR", url: baseURL ?? "http://localhost:3200" }]);
			await page.goto(path);
			await expect(page.getByRole("link", { name: "Continuar com o Google" })).toBeVisible();
		});
	}
});

test("sign-up name rules: live capitalization, length and spaces", async ({ page }) => {
	await page.goto("/register");
	const name = page.getByLabel("Name");

	await name.pressSequentially("álvaro");
	await expect(name).toHaveValue("Álvaro");

	// Too short: the client refuses it before any request.
	await name.fill("abc");
	await page.getByLabel("Email").fill(uniqueEmail("short"));
	await page.getByLabel(/^Password/).fill(testPassword);
	await page.getByLabel("Confirm password").fill(testPassword);
	await page.locator('main form button[type="submit"]').click();
	await expect(page.getByText(nameError)).toBeVisible();
	await expect(page).toHaveURL(/\/register$/);

	// Too long: the input stops at 24 characters (the server rejects more).
	await name.fill("Bartholomew Fitzgerald Jr");
	await expect(name).toHaveValue("Bartholomew Fitzgerald J");

	// Spaces are trimmed and collapsed before counting and saving.
	await name.fill("  ana    maria  ");
	await page.locator('main form button[type="submit"]').click();
	await expect(page).toHaveURL(/\/(dashboard|email\/verify)$/);
	await expect(page.getByRole("link", { name: "Ana maria" })).toBeVisible();
});

test("PT-BR name message", async ({ page, context, baseURL }) => {
	await context.addCookies([{ name: "lang", value: "pt-BR", url: baseURL ?? "http://localhost:3200" }]);
	await page.goto("/register");
	await page.getByLabel("Nome").fill("Bia");
	await page.getByLabel("Nome").blur();
	await expect(page.getByText("Seu nome deve ter de 4 a 24 caracteres.")).toBeVisible();
});

test.describe("admin feature flags", () => {
	test.skip(!canRunAppCommands, "needs the compose app container or E2E_APP_BIN for users:make-admin");

	async function setEmailVerification(page: Page, on: boolean): Promise<void> {
		await page.goto("/admin/flags");
		const toggle = page.getByRole("switch", { name: "Email verification" });
		if ((await toggle.getAttribute("aria-checked")) !== String(on)) {
			await toggle.click();
			await expect(page.getByRole("status")).toContainText("The flag was updated.");
		}
		await expect(toggle).toHaveAttribute("aria-checked", String(on));
	}

	test("email_verification changes what sign-up does", async ({ browser }) => {
		const adminContext = await browser.newContext();
		const admin = await adminContext.newPage();
		await createAdmin(admin, "Flora Admin", uniqueEmail("flags-admin"));

		try {
			await admin.goto("/admin/flags");
			await expect(admin.getByRole("heading", { name: "Feature flags" })).toBeVisible();

			// The admin 2FA requirement is locked on.
			await expect(admin.getByRole("switch", { name: "Two-factor authentication for admins" })).toBeDisabled();

			await setEmailVerification(admin, true);
			await expect(admin.getByRole("row").filter({ hasText: "Email verification" }).first()).toContainText(
				"Off → On",
			);

			const on = await browser.newContext();
			expect(await register(await on.newPage(), "Vera Verify", uniqueEmail("flag-on"), testPassword)).toBe(
				"verify",
			);
			await on.close();

			await setEmailVerification(admin, false);

			const off = await browser.newContext();
			expect(await register(await off.newPage(), "Otto Direct", uniqueEmail("flag-off"), testPassword)).toBe(
				"dashboard",
			);
			await off.close();
		} finally {
			await setEmailVerification(admin, false);
			await adminContext.close();
		}
	});
});
