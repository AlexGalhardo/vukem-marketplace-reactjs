import { type Browser, type Cookie, expect, type Page, test } from "@playwright/test";

import { demoPassword, register, sessionCookies, testPassword, uniqueEmail } from "./support";

// Buyer ↔ seller chat against the seeded stack, with two browser contexts
// live at once: a fresh buyer opens a conversation from a demo product,
// seller@example.com answers over the WebSocket without reloading, and the
// typing indicator, read receipts and header badge follow.
const product = "/p/impact-drill-650-w";
const sellerPassword = process.env.E2E_SELLER_PASSWORD ?? demoPassword;

// One seller login for the file: logging in per test trips the login
// throttle (5 per minute per email and IP).
let sellerCookies: Cookie[] = [];

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ browser }) => {
	sellerCookies = await sessionCookies(await browser.newContext(), "seller@example.com", sellerPassword);
});

async function sellerPage(browser: Browser): Promise<Page> {
	const context = await browser.newContext();
	await context.addCookies(sellerCookies);
	return context.newPage();
}

test("buyer and seller chat live", async ({ browser }) => {
	// Letters only: a long digit run would read as a phone number.
	const tag = Math.random()
		.toString(36)
		.replace(/[^a-z]/g, "")
		.slice(0, 8);
	const question = `Does it come with a carrying case? Ref ${tag}`;
	const answer = `Yes, a hard case is included. Ref ${tag}`;

	const buyerContext = await browser.newContext();
	const buyer = await buyerContext.newPage();
	await register(buyer, "Carla Chat", uniqueEmail("chat-buyer"), testPassword);
	await buyer.goto(product);
	await buyer.getByRole("button", { name: "Chat with the seller" }).click();
	await expect(buyer).toHaveURL(/\/messages\/\d+$/);
	const conversation = new URL(buyer.url()).pathname;
	await expect(buyer.getByTestId("chat-status")).toHaveAttribute("data-status", "live");

	const composer = buyer.getByLabel("Your message");
	// The no-contact rule answers before the request leaves the browser.
	await composer.fill("Call me at (11) 98765-4321 for a deal");
	await buyer.getByRole("button", { name: "Send" }).click();
	await expect(buyer.getByRole("alert")).toContainText("keep the conversation on the platform");

	await composer.fill(question);
	await composer.press("Enter");
	const buyerLog = buyer.getByRole("log");
	await expect(buyerLog).toContainText(question);
	await expect(composer).toHaveValue("");

	// The seller, in a second context, sees the unread badge, then opens the
	// conversation: it is marked read and the buyer sees "Seen" live.
	const seller = await sellerPage(browser);
	await seller.goto("/seller/messages");
	await expect(seller.getByTestId("chat-unread")).toBeVisible();
	const item = seller.getByRole("link").filter({ hasText: "Carla" }).first();
	await expect(item).toContainText(question);
	await item.click();
	await expect(seller).toHaveURL(new RegExp(`${conversation}$`));
	await expect(seller.getByRole("log")).toContainText(question);
	await expect(seller.getByTestId("chat-status")).toHaveAttribute("data-status", "live");
	await expect(buyer.getByText("Seen")).toBeVisible();

	// Both sides are online and typing shows on the other side.
	await expect(buyer.getByText("Online")).toBeVisible();
	const reply = seller.getByLabel("Your message");
	await reply.pressSequentially("Yes", { delay: 30 });
	await expect(buyer.getByText(/is typing/)).toBeVisible();

	await reply.fill(answer);
	await seller.getByRole("button", { name: "Send" }).click();
	await expect(buyerLog).toContainText(answer);
	await expect(buyer.getByText(/is typing/)).toHaveCount(0);

	// Reloading the buyer page keeps the history (stored in Postgres).
	await buyer.reload();
	await expect(buyer.getByRole("log")).toContainText(answer);

	// The buyer's inbox lists the shop's conversation.
	await buyer.goto("/messages");
	await expect(buyer.getByRole("link").filter({ hasText: answer })).toBeVisible();

	await buyerContext.close();
	await seller.context().close();
});

test("missed messages arrive through the fallback and after a reconnect", async ({ browser }) => {
	const tag = Math.random()
		.toString(36)
		.replace(/[^a-z]/g, "")
		.slice(0, 8);
	const missed = `Sent while your socket was down ${tag}`;
	// Three backoffs, the SSE attempt and the 30 s socket retry.
	test.setTimeout(120_000);

	const buyerContext = await browser.newContext();
	const buyer = await buyerContext.newPage();

	// A proxy in front of the socket and the SSE stream lets the test cut
	// both, like a network or proxy that blocks WebSockets.
	let blocked = false;
	let socket: { close: () => Promise<void> } | undefined;
	await buyer.routeWebSocket(/\/broadcast\/ws$/, (ws) => {
		if (blocked) {
			void ws.close();
			return;
		}
		socket = ws;
		ws.connectToServer();
	});
	await buyer.route(/\/broadcast\/sse/, (route) => (blocked ? route.abort() : route.continue()));

	await register(buyer, "Otto Offline", uniqueEmail("chat-offline"), testPassword);
	await buyer.goto(product);
	await buyer.getByRole("button", { name: "Chat with the seller" }).click();
	await expect(buyer).toHaveURL(/\/messages\/\d+$/);
	const conversation = new URL(buyer.url()).pathname;
	await buyer.getByLabel("Your message").fill(`Hello from Otto ${tag}`);
	await buyer.getByRole("button", { name: "Send" }).click();
	const status = buyer.getByTestId("chat-status");
	await expect(status).toHaveAttribute("data-status", "live");

	// Cut realtime: three failed reconnects, a failed SSE stream, then the
	// page polls the catch-up endpoint.
	blocked = true;
	await socket?.close();
	await expect(status).toHaveAttribute("data-status", "fallback", { timeout: 20_000 });

	const seller = await sellerPage(browser);
	await seller.goto(conversation);
	await seller.getByLabel("Your message").fill(missed);
	await seller.getByRole("button", { name: "Send" }).click();
	await expect(seller.getByRole("log")).toContainText(missed);
	await expect(buyer.getByRole("log")).toContainText(missed, { timeout: 10_000 });

	// Realtime comes back: the periodic retry reconnects the socket.
	blocked = false;
	await expect(status).toHaveAttribute("data-status", "live", { timeout: 45_000 });

	await buyerContext.close();
	await seller.context().close();
});

test("the seller has no chat button on their own product", async ({ browser }) => {
	const page = await sellerPage(browser);
	await page.goto(product);
	await expect(page.getByRole("button", { name: "Chat with the seller" })).toHaveCount(0);
	await page.context().close();
});
