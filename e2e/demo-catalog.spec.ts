import { expect, test } from "@playwright/test";

// The demo catalog's simulated activity (demo:catalog, run by db:seed): a
// seeded product shows the demo buyers' reviews and the seller's answers.
// The slug is deterministic (seed 20261018): the first demo product in slug
// order, the head of the simulated sales; tests/integration/seed_test.go
// asserts it has reviews and answered questions.
const showcase = "/p/29-mountain-bike-21-speed-frame-17-disc-brakes-atletica-dm03106";

test("a seeded product shows reviews and answered questions", async ({ page }) => {
	await page.goto(showcase);
	await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

	const reviews = page.locator("#reviews");
	await expect(reviews.getByText(/^\d+ ratings?$/).first()).toBeVisible();
	await expect(reviews.getByText(/^\d[.,]\d out of 5$/)).toBeAttached();
	await expect(reviews.locator("article").first()).toBeVisible();
	await expect(reviews.getByText("Verified purchase").first()).toBeVisible();

	const answered = page.getByTestId("question-list").getByRole("listitem").filter({ hasText: "Seller answer" });
	await expect(answered.first()).toBeVisible();
});
