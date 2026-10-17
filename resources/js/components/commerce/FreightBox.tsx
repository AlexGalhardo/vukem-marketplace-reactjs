import { type FormEvent, type JSX, useId, useState } from "react";

import { useT } from "../../lib/i18n";
import { formatMoney } from "../../lib/money";

interface Props {
	slug: string;
}

interface Quote {
	uf: string;
	priceCents: number;
	days: number;
}

type State =
	| { kind: "idle" }
	| { kind: "loading" }
	| { kind: "done"; quote: Quote }
	| { kind: "error"; key: "invalid" | "error" };

// FreightBox prices shipping of one unit to a CEP (GET /freight), for the
// product page. The checkout prices shipping again per seller.
export default function FreightBox({ slug }: Props): JSX.Element {
	const { t, locale } = useT();
	const id = useId();
	const [cep, setCep] = useState("");
	const [state, setState] = useState<State>({ kind: "idle" });

	const submit = async (e: FormEvent) => {
		e.preventDefault();
		const digits = cep.replace(/\D/g, "");
		if (digits.length !== 8) {
			setState({ kind: "error", key: "invalid" });
			return;
		}
		setState({ kind: "loading" });
		try {
			const res = await fetch(`/freight?product=${encodeURIComponent(slug)}&cep=${digits}`, {
				headers: { Accept: "application/json" },
			});
			if (res.status === 422) {
				setState({ kind: "error", key: "invalid" });
				return;
			}
			if (!res.ok) throw new Error(String(res.status));
			setState({ kind: "done", quote: (await res.json()) as Quote });
		} catch {
			setState({ kind: "error", key: "error" });
		}
	};

	return (
		<form
			onSubmit={submit}
			aria-labelledby={`${id}-title`}
			className="flex flex-col gap-2 rounded-md border border-line p-4"
		>
			<h2 id={`${id}-title`} className="text-sm font-medium text-ink">
				{t("cart.freight.title")}
			</h2>
			<label htmlFor={id} className="text-sm text-muted">
				{t("cart.freight.cep")}
			</label>
			<div className="flex gap-2">
				<input
					id={id}
					inputMode="numeric"
					autoComplete="postal-code"
					placeholder="00000-000"
					maxLength={9}
					value={cep}
					onChange={(e) => setCep(e.target.value)}
					aria-describedby={`${id}-result`}
					className="h-10 w-32 rounded-md border border-line bg-card px-3 text-sm text-ink"
				/>
				<button
					type="submit"
					disabled={state.kind === "loading"}
					className="h-10 rounded-md border border-action-500 px-4 text-sm font-medium text-action-600 hover:bg-action-500/10 disabled:opacity-50"
				>
					{t("cart.freight.calculate")}
				</button>
			</div>
			<p id={`${id}-result`} aria-live="polite" className="text-sm">
				{state.kind === "done" && (
					<span className="text-success-600">
						{t("cart.freight.result", {
							price: formatMoney(state.quote.priceCents, locale),
							uf: state.quote.uf,
							days: state.quote.days,
						})}
					</span>
				)}
				{state.kind === "error" && <span className="text-danger-600">{t(`cart.freight.${state.key}`)}</span>}
			</p>
		</form>
	);
}
