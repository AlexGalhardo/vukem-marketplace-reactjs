import { Link, router, usePage } from "@inertiajs/react";
import { type JSX, useState } from "react";

import { useT } from "../../lib/i18n";
import type { SharedProps } from "../../types";
import { ChatIcon } from "../icons";
import { secondaryButton } from "../ui";

// ChatWithSeller opens (or reopens) the buyer's conversation about a
// product. Hidden while the chat flag is off and on the owner's own
// products; guests get a link to log in.
export default function ChatWithSeller({ slug, own }: { slug: string; own: boolean }): JSX.Element | null {
	const { t } = useT();
	const { auth, chat } = usePage<SharedProps>().props;
	const [busy, setBusy] = useState(false);

	if (!chat?.enabled || own) return null;

	if (!auth?.user) {
		return (
			<Link href="/login" className="text-sm text-action-600 hover:underline">
				{t("chat.cta_login")}
			</Link>
		);
	}

	return (
		<div className="flex flex-col gap-1.5">
			<button
				type="button"
				disabled={busy}
				onClick={() =>
					router.post(
						"/messages",
						{ product: slug },
						{ onStart: () => setBusy(true), onFinish: () => setBusy(false) },
					)
				}
				className={`${secondaryButton} gap-2`}
			>
				<ChatIcon className="size-5" />
				{t("chat.cta")}
			</button>
			<p className="text-xs text-muted">{t("chat.cta_hint")}</p>
		</div>
	);
}
