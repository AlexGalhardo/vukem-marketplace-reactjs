import { expect, type Page, test } from "@playwright/test";

import { appCommand, canRunAppCommands, demoPassword, logIn, register, testPassword, uniqueEmail } from "./support";

// Reviews on the seeded stack: a fresh buyer gets a paid order through the
// dev:paid-order command (no payment provider), reviews the demo drill,
// and the product page shows the review with its badge and the new
// average; the demo seller (seller@example.com owns the drill) replies and
// another buyer votes it helpful.
const product = "/p/impact-drill-650-w";
const slug = "impact-drill-650-w";

test.describe.configure({ mode: "serial" });
test.skip(!canRunAppCommands, "needs the compose app container or E2E_APP_BIN for dev:paid-order");

async function ratingCount(page: Page): Promise<number> {
	const summary = page.locator("#reviews").getByText(/^\d+ ratings?$/);
	if ((await summary.count()) === 0) return 0;
	return Number.parseInt((await summary.first().textContent()) ?? "0", 10);
}

// Letters only (a long digit run reads as a phone number to the filter),
// unique per worker: the two Playwright projects run this file at once.
const letters = (): string =>
	Math.random()
		.toString(36)
		.replace(/[^a-z]/g, "")
		.slice(0, 6);
const title = `Solid drill ${letters()}${letters()}`;

test("an eligible buyer reviews and the product page shows it", async ({ page }) => {
	const email = uniqueEmail("reviewer");
	await register(page, "Rita Reviewer", email, testPassword);

	await page.goto(product);
	const before = await ratingCount(page);
	await expect(page.getByRole("link", { name: "Write a review" })).toHaveCount(0);

	appCommand("dev:paid-order", email, slug);
	await page.goto(product);
	await page.getByRole("link", { name: "Write a review" }).first().click();
	await expect(page.getByRole("heading", { level: 1, name: "Review your purchase" })).toBeVisible();

	// Client-side validation: stars are required, links are refused.
	await page.getByLabel("Your review (optional)").fill("Cheaper at https://example.com");
	await page.getByRole("button", { name: "Publish review" }).click();
	await expect(page.getByText("Choose from 1 to 5 stars.")).toBeVisible();
	await expect(page.getByText("Links, emails and phone numbers are not allowed.", { exact: true })).toBeVisible();

	await page.locator("label").filter({ hasText: "4 of 5 stars" }).click();
	await page.getByLabel("Title (optional)").fill(title);
	await page.getByLabel("Your review (optional)").fill("Strong, light and the battery lasts.");
	await page.getByLabel("Yes", { exact: true }).check();
	await page.getByRole("button", { name: "Publish review" }).click();

	await expect(page).toHaveURL(new RegExp(`${product}$`));
	await expect(page.getByText("Thanks! Your review is published.")).toBeVisible();

	const review = page.getByRole("article", { name: title });
	await expect(review).toBeVisible();
	await expect(review.getByText("Verified purchase")).toBeVisible();
	await expect(review.getByText("Recommends this product")).toBeVisible();
	await expect(review.getByText(/^Rita · /)).toBeVisible();
	// Other workers may review the same product meanwhile: at least ours counts.
	expect(await ratingCount(page)).toBeGreaterThan(before);
	await expect(page.locator("#reviews").getByText(/^\d[.,]\d out of 5$/)).toBeAttached();

	// One review per purchase: the write link is gone.
	await expect(page.getByRole("link", { name: "Write a review" })).toHaveCount(0);
});

test("the seller replies once and another buyer votes it helpful", async ({ page, browser }) => {
	await logIn(page, "seller@example.com", demoPassword);
	await expect(page).not.toHaveURL(/\/login/);
	await page.goto(product);
	const review = page.getByRole("article", { name: title });
	await review.getByLabel("Reply to this review").fill("Thanks for buying with us!");
	await review.getByRole("button", { name: "Publish reply" }).click();
	await expect(review.getByText("Reply from the seller")).toBeVisible();
	await expect(review.getByText("Thanks for buying with us!")).toBeVisible();
	await expect(review.getByLabel("Reply to this review")).toHaveCount(0);

	const context = await browser.newContext();
	const voter = await context.newPage();
	await register(voter, "Vera Voter", uniqueEmail("voter"), testPassword);
	await voter.goto(product);
	const seen = voter.getByRole("article", { name: title });
	await seen.getByRole("button", { name: "Helpful" }).click();
	await expect(seen.getByText("You found this helpful")).toBeVisible();
	await expect(seen.getByText("1 person found this helpful")).toBeVisible();
	await expect(seen.getByRole("button", { name: "Helpful" })).toHaveCount(0);
	await context.close();
});

test("guests see reviews but no write link, vote button or reply form", async ({ page }) => {
	await page.goto(product);
	const review = page.getByRole("article", { name: title });
	await expect(review).toBeVisible();
	await expect(review.getByRole("button", { name: "Helpful" })).toHaveCount(0);
	await expect(page.getByLabel("Reply to this review")).toHaveCount(0);

	await page.getByLabel("Sort by").selectOption("helpful");
	await expect(page).toHaveURL(/rsort=helpful/);
	await expect(page.locator("#reviews article").first()).toBeVisible();
});
