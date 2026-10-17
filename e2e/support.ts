import { execFileSync } from "node:child_process";
import { createHmac } from "node:crypto";

import { type BrowserContext, type Cookie, expect, type Page } from "@playwright/test";

// Shared helpers for the e2e specs. Demo accounts come from `db:seed`
// (password SEED_PASSWORD, default below).

export const demoPassword = process.env.E2E_SEED_PASSWORD ?? "marketplace-demo-password";
export const testPassword = "Correct-Horse-Battery-9";

export function uniqueEmail(prefix: string): string {
	return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;
}

// totp computes the RFC 6238 code (SHA-1, 6 digits, 30 s) of a base32
// secret at time `at`, like an authenticator app.
export function totp(secret: string, at: number): string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	let bits = "";
	for (const char of secret.replace(/=+$/, "").toUpperCase()) {
		bits += alphabet.indexOf(char).toString(2).padStart(5, "0");
	}
	const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((byte) => Number.parseInt(byte, 2)));
	const counter = Buffer.alloc(8);
	counter.writeBigUInt64BE(BigInt(Math.floor(at / 30_000)));
	const mac = createHmac("sha1", key).update(counter).digest();
	const offset = (mac.at(-1) ?? 0) & 0x0f;
	const value = (mac.readUInt32BE(offset) & 0x7fffffff) % 1_000_000;
	return value.toString().padStart(6, "0");
}

export async function logIn(page: Page, email: string, password: string): Promise<void> {
	await page.goto("/login");
	await page.getByLabel("Email").fill(email);
	await page.getByLabel("Password").fill(password);
	await page.getByRole("button", { name: /log in/i }).click();
}

// sessionCookies logs in once in a throwaway context and returns its
// cookies: logging in before every test trips the login throttle (5 per
// minute per email and IP).
export async function sessionCookies(context: BrowserContext, email: string, password: string): Promise<Cookie[]> {
	const page = await context.newPage();
	await logIn(page, email, password);
	await expect(page).not.toHaveURL(/\/login/);
	const { cookies } = await context.storageState();
	await context.close();
	return cookies;
}

// mailpitLink waits for a mail to `to` in the compose Mailpit and returns
// the first app link whose path starts with pathPrefix.
export async function mailpitLink(to: string, pathPrefix: string): Promise<string> {
	const api = process.env.E2E_MAILPIT_URL ?? "http://localhost:8035";
	for (let attempt = 0; attempt < 30; attempt++) {
		const search = await fetch(`${api}/api/v1/search?query=${encodeURIComponent(`to:${to}`)}`);
		const { messages } = (await search.json()) as { messages: { ID: string }[] };
		const first = messages[0];
		if (first) {
			const msg = await fetch(`${api}/api/v1/message/${first.ID}`);
			const { Text } = (await msg.json()) as { Text: string };
			const path = Text.match(new RegExp(`https?://[^\\s"]+(${pathPrefix}[^\\s")]*)`))?.[1];
			if (path) return path;
		}
		await new Promise((resolve) => setTimeout(resolve, 1_000));
	}
	throw new Error(`no mail with a ${pathPrefix} link reached ${to}`);
}

// canRunAppCommands reports whether appCommand can reach the app: the
// compose container, or a local binary named by E2E_APP_BIN.
export const canRunAppCommands = process.env.E2E_DOCKER !== "0" || Boolean(process.env.E2E_APP_BIN);

// appCommand runs a command of the compose app container (e.g.
// users:make-admin). Set E2E_DOCKER=0 when the stack is not compose, and
// E2E_APP_BIN to the app binary of a local stack (it inherits this
// process's environment, so run Playwright with the app's variables).
export function appCommand(...args: string[]): void {
	const bin = process.env.E2E_APP_BIN;
	if (bin) {
		execFileSync(bin, args, { stdio: "pipe" });
		return;
	}
	execFileSync("docker", ["compose", "exec", "-T", "app", "/app/app", ...args], {
		stdio: "pipe",
		env: { ...process.env, MSYS_NO_PATHCONV: "1" },
	});
}

// register signs up through the form. It lands logged in: on the dashboard,
// or on the verification notice while the email_verification flag is on;
// it returns which.
export async function register(
	page: Page,
	name: string,
	email: string,
	password: string,
): Promise<"dashboard" | "verify"> {
	await page.goto("/register");
	await page.getByLabel("Name").fill(name);
	await page.getByLabel("Email").fill(email);
	await page.getByLabel(/^Password/).fill(password);
	await page.getByLabel("Confirm password").fill(password);
	await page.locator('main form button[type="submit"]').click();
	await expect(page).toHaveURL(/\/(dashboard|email\/verify)$/);
	return page.url().endsWith("/email/verify") ? "verify" : "dashboard";
}

// createAdmin registers email with a verified address and confirmed 2FA,
// then promotes it with the CLI (the only way to create admins). The page
// stays logged in as the admin.
export async function createAdmin(page: Page, name: string, email: string): Promise<void> {
	await register(page, name, email, testPassword);
	await verifyEmail(page, email);

	await page.goto("/user/two-factor");
	await page.getByRole("button", { name: "Enable two-factor authentication" }).click();
	const secret = (await page.locator("code").textContent())?.trim() ?? "";
	await page.getByLabel("Code").fill(totp(secret, Date.now()));
	await page.getByRole("button", { name: "Confirm" }).click();
	await expect(page.getByText("Store these recovery codes somewhere safe.")).toBeVisible();

	appCommand("users:make-admin", email);
}

// verifyEmail asks for a verification mail and follows its link (Mailpit),
// so the account keeps working when email_verification is switched on.
export async function verifyEmail(page: Page, email: string): Promise<void> {
	await page.goto("/account");
	await page.getByRole("button", { name: /verification link again/i }).click();
	await page.goto(await mailpitLink(email, "/email/verify/"));
	await expect(page).toHaveURL(/\/dashboard$/);
}
