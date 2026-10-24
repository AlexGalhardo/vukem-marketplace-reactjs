// Records the GIFs of the README against the running compose stack
// (http://localhost:3200, demo data seeded): Playwright video, ffmpeg GIF.
//
//   bun scripts/record-readme-gifs.ts [register|search|theme|checkout|chat|tests]...
//
// Output: docs/media/<name>.gif. Needs `bunx playwright install chromium`
// and ffmpeg on the PATH.
import { execFileSync, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readdirSync, renameSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { type Browser, type BrowserContext, chromium, type Page } from "@playwright/test";

const base = process.env.BASE_URL ?? "http://localhost:3200";
const out = join(import.meta.dir, "..", "docs", "media");
const demoPassword = process.env.SEED_PASSWORD || "marketplace-demo-password";
const password = "Correct-Horse-9-Battery";
const product = "/p/impact-drill-650-w";
const size = { width: 1100, height: 650 };

const pause = (page: Page, ms = 900): Promise<void> => page.waitForTimeout(ms);

async function record(
	browser: Browser,
	name: string,
	scenario: (page: Page, newContext: () => Promise<BrowserContext>) => Promise<void>,
	prepare?: (page: Page) => Promise<void>,
): Promise<void> {
	const dir = mkdtempSync(join(tmpdir(), `gif-${name}-`));
	// prepare runs before the recording (login, setup) and its session is kept.
	let storageState: Awaited<ReturnType<BrowserContext["storageState"]>> | undefined;
	if (prepare) {
		const warm = await browser.newContext({ baseURL: base, viewport: size });
		await prepare(await warm.newPage());
		storageState = await warm.storageState();
		await warm.close();
	}
	const context = await browser.newContext({
		baseURL: base,
		viewport: size,
		storageState,
		recordVideo: { dir, size },
	});
	const page = await context.newPage();
	const extra: BrowserContext[] = [];
	try {
		await scenario(page, async () => {
			const c = await browser.newContext({ baseURL: base, viewport: size });
			extra.push(c);
			return c;
		});
		await pause(page, 1200);
	} finally {
		await context.close();
		for (const c of extra) await c.close();
	}
	const video = readdirSync(dir).find((f) => f.endsWith(".webm"));
	if (!video) throw new Error(`${name}: no video recorded`);
	mkdirSync(out, { recursive: true });
	const gif = join(out, `${name}.gif`);
	execFileSync(
		"ffmpeg",
		[
			"-y",
			"-loglevel",
			"error",
			"-i",
			join(dir, video),
			"-vf",
			"fps=10,scale=900:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=96[p];[b][p]paletteuse=dither=bayer:bayer_scale=4",
			gif,
		],
		{ stdio: "inherit" },
	);
	rmSync(dir, { recursive: true, force: true });
	console.log(`wrote ${gif}`);
}

async function logIn(page: Page, email: string, pass: string): Promise<void> {
	await page.goto("/login");
	await page.getByLabel("Email").fill(email);
	await page.getByLabel("Password").fill(pass);
	await page.getByRole("button", { name: /log in/i }).click();
	await page.waitForURL(/\/(dashboard|seller|account)/);
}

const scenarios: Record<string, (b: Browser) => Promise<void>> = {
	register: (b) =>
		record(b, "register", async (page) => {
			await page.goto("/register");
			await pause(page);
			await page.getByLabel("Name").pressSequentially("Carla Gifs", { delay: 70 });
			await page
				.getByLabel("Email")
				.pressSequentially(`carla.gifs.${Date.now().toString(36)}@example.com`, { delay: 40 });
			const pw = page.getByLabel(/^Password/);
			for (const chunk of ["abcdefgh", "A", "7", "-"]) {
				await pw.pressSequentially(chunk, { delay: 70 });
				await pause(page, 700);
			}
			await page.getByRole("button", { name: "Show password" }).first().click();
			await pause(page, 1200);
			await page.getByLabel("Confirm password").fill("abcdefghA7-");
			await page.locator('main form button[type="submit"]').click();
			await page.waitForURL(/\/(dashboard|email\/verify)$/);
		}),

	search: (b) =>
		record(
			b,
			"search",
			async (page) => {
			await page.goto("/");
			await pause(page);
			const box = page.getByRole("combobox");
			await box.pressSequentially("blue", { delay: 140 });
			await page.getByRole("option").first().waitFor();
			await pause(page, 1600);
			await box.press("ArrowDown");
			await box.press("ArrowDown");
			await pause(page, 900);
			await box.press("Enter");
			await page.waitForURL(/\/p\//);
			await pause(page);
			await page.getByRole("button", { name: "Add to cart" }).click();
			await pause(page, 1000);
			await page.goto("/cart");
			await pause(page, 1500);
			},
			(page) => logIn(page, "buyer@example.com", demoPassword),
		),

	theme: (b) =>
		record(b, "theme", async (page) => {
			await page.goto("/");
			await pause(page, 1200);
			const toggle = page.getByRole("button", { name: /theme|dark|light/i }).first();
			await toggle.click();
			await pause(page, 1500);
			await page.goto(product);
			await pause(page, 1500);
			await page.getByRole("button", { name: /theme|dark|light/i }).first().click();
			await pause(page, 1500);
		}),

	checkout: (b) =>
		record(
			b,
			"checkout",
			async (page) => {
				await page.goto(product);
				await pause(page, 700);
				await page.getByRole("button", { name: "Add to cart" }).click();
				await pause(page, 700);
				await page.goto("/cart");
				await pause(page, 1000);
				await page.goto("/checkout");
				await pause(page, 900);
				await page.getByRole("button", { name: "Deliver here" }).click();
				await page.waitForURL(/\/checkout\/payment$/);
				await pause(page, 1800);
			},
			async (page) => {
				await logIn(page, "buyer@example.com", demoPassword);
				await page.goto("/account/addresses");
				if (await page.getByRole("button", { name: "Save address" }).isVisible()) {
					await page.getByLabel("Recipient").fill("Demo Buyer");
					await page.getByLabel("CEP").fill("01310-100");
					await page.getByLabel("Street").fill("Avenida Paulista");
					await page.getByLabel("Number").fill("1000");
					await page.getByLabel("District").fill("Bela Vista");
					await page.getByLabel("City").fill("Sao Paulo");
					await page.getByLabel("State").selectOption("SP");
					await page.getByRole("button", { name: "Save address" }).click();
					await page.waitForTimeout(800);
				}
			},
		),

	// The terminal is a page that replays the real output of the command.
	tests: (b) =>
		record(b, "tests", async (page) => {
			const args = ["test", "-count=1", "-v", "./tests/catalog/", "-run", "Search|Suggest|Home"];
			const result = spawnSync("go", args, { encoding: "utf8" });
			const lines = (result.stdout || "").split(String.fromCharCode(10)).map((l) => l.trim()).filter((l) => l !== "");
			await page.setContent(
				`<body style="margin:0;background:#0d1117;color:#c9d1d9;font:15px/1.5 Consolas,monospace;padding:24px"><div id="t"></div></body>`,
			);
			const write = (text: string): Promise<void> =>
				page.evaluate((t) => {
					const d = document.createElement("div");
					d.textContent = t;
					if (/^(--- PASS|ok|PASS)/.test(t)) d.style.color = "#3fb950";
					if (/FAIL/.test(t)) d.style.color = "#f85149";
					document.getElementById("t")?.append(d);
					window.scrollTo(0, document.body.scrollHeight);
				}, text);
			await write(`$ go ${args.join(" ")}`);
			await pause(page, 700);
			for (const line of lines.slice(-26)) {
				await write(line);
				await page.waitForTimeout(110);
			}
		}),

	chat: (b) =>
		record(b, "chat", async (buyer, newContext) => {
			const email = `chat.gifs.${Date.now().toString(36)}@example.com`;
			await buyer.goto("/register");
			await buyer.getByLabel("Name").fill("Carla Chat");
			await buyer.getByLabel("Email").fill(email);
			await buyer.getByLabel(/^Password/).fill(password);
			await buyer.getByLabel("Confirm password").fill(password);
			await buyer.locator('main form button[type="submit"]').click();
			await buyer.waitForURL(/\/(dashboard|email\/verify)$/);
			await buyer.goto(product);
			await buyer.getByRole("button", { name: "Chat with the seller" }).click();
			await buyer.waitForURL(/\/messages\/\d+$/);
			const conversation = new URL(buyer.url()).pathname;
			await pause(buyer);
			const composer = buyer.getByLabel("Your message");
			await composer.pressSequentially("Hi! Does the drill come with a case?", { delay: 60 });
			await composer.press("Enter");
			await pause(buyer, 1200);

			const sellerPage = await (await newContext()).newPage();
			await logIn(sellerPage, "seller@example.com", demoPassword);
			await sellerPage.goto(conversation);
			await sellerPage.getByLabel("Your message").pressSequentially("Yes, a hard case is included.", { delay: 40 });
			await pause(buyer, 800);
			await sellerPage.getByLabel("Your message").press("Enter");
			await buyer.getByText("hard case is included").waitFor();
			await pause(buyer, 2000);
		}),
};

const wanted = process.argv.slice(2).filter((n) => n in scenarios);
const browser = await chromium.launch({ slowMo: 60 });
try {
	for (const name of wanted.length > 0 ? wanted : Object.keys(scenarios)) {
		const run = scenarios[name];
		if (run) await run(browser);
	}
} finally {
	await browser.close();
}
