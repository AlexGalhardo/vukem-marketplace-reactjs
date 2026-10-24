import { expect, test } from "@playwright/test";

import { demoPassword, logIn, register, testPassword, uniqueEmail } from "./support";

// Questions and answers against the seeded stack: a fresh buyer asks on a
// demo product (a new account per run keeps clear of the per-user ask
// limit), seller@example.com answers from the seller inbox and a guest
// sees the answer on the product page.
const product = "/p/impact-drill-650-w";
const sellerPassword = process.env.E2E_SELLER_PASSWORD ?? demoPassword;

test("ask, answer from the seller inbox, see the answer", async ({ browser }) => {
	// Letters only: a long digit run would read as a phone number.
	const tag = Math.random()
		.toString(36)
		.replace(/[^a-z]/g, "")
		.slice(0, 8);
	const question = `Does it come with a carrying case? Ref ${tag}`;
	const answer = `Yes, a hard case is included. Ref ${tag}`;

	const buyer = await browser.newPage();
	await register(buyer, "Quentin Questioner", uniqueEmail("qna-buyer"), testPassword);
	await buyer.goto(product);

	const ask = buyer.getByLabel("Ask the seller");
	// The no-contact rule answers before the request leaves the browser.
	await ask.fill("Call me at (11) 98765-4321 for a deal");
	await buyer.getByRole("button", { name: "Ask", exact: true }).click();
	await expect(buyer.getByText(/keep the conversation on the platform/)).toBeVisible();

	await ask.fill(question);
	await buyer.getByRole("button", { name: "Ask", exact: true }).click();
	await expect(buyer.getByText("Your question was sent to the seller.").first()).toBeVisible();
	const item = buyer.getByTestId("question-list").getByRole("listitem").filter({ hasText: question });
	await expect(item).toContainText("Quentin asked");
	await expect(item).toContainText("Waiting for the seller's answer");
	await buyer.close();

	const seller = await browser.newPage();
	await logIn(seller, "seller@example.com", sellerPassword);
	await expect(seller).not.toHaveURL(/\/login/);
	await seller.goto("/seller/questions");
	const card = seller.getByRole("listitem").filter({ hasText: question });
	await card.getByLabel("Your answer").fill(answer);
	await card.getByRole("button", { name: "Publish answer" }).click();
	await expect(seller.getByText("Your answer was published.")).toBeVisible();
	await expect(seller.getByRole("listitem").filter({ hasText: question })).toHaveCount(0);
	await seller.goto("/seller/questions?filter=answered");
	await expect(seller.getByRole("listitem").filter({ hasText: question })).toBeVisible();
	await seller.close();

	const guest = await browser.newPage();
	await guest.goto(product);
	const answered = guest.getByTestId("question-list").getByRole("listitem").filter({ hasText: question });
	await expect(answered).toContainText(answer);
	await expect(answered).toContainText("Seller answer");
	await expect(guest.getByRole("link", { name: "Log in to ask the seller" })).toBeVisible();
	await guest.close();
});
