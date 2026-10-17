import type { JSX } from "react";

import { useT } from "../../lib/i18n";
import Field from "../Field";

export interface ShopForm {
	name: string;
	description: string;
	ship_from_uf: string;
}

const controlClass =
	"w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus:border-action-500 focus:outline-none";

// ShopFields are the shop name, description and shipping origin inputs
// shared by the open-shop and settings forms.
export default function ShopFields({
	data,
	errors,
	ufs,
	set,
}: {
	data: ShopForm;
	errors: Partial<Record<keyof ShopForm, string>>;
	ufs: string[];
	set: (key: keyof ShopForm, value: string) => void;
}): JSX.Element {
	const { t } = useT();

	return (
		<>
			<Field
				label={t("seller.shop.name")}
				required
				maxLength={60}
				value={data.name}
				onChange={(e) => set("name", e.target.value)}
				error={errors.name}
			/>
			<div className="flex flex-col gap-1">
				<label htmlFor="shop-description" className="text-sm font-medium text-ink">
					{t("seller.shop.description")}
				</label>
				<textarea
					id="shop-description"
					rows={4}
					maxLength={1000}
					value={data.description}
					onChange={(e) => set("description", e.target.value)}
					className={controlClass}
				/>
				{errors.description && <p className="text-sm text-danger-600">{errors.description}</p>}
			</div>
			<div className="flex flex-col gap-1">
				<label htmlFor="shop-uf" className="text-sm font-medium text-ink">
					{t("seller.shop.ship_from")}
				</label>
				<select
					id="shop-uf"
					value={data.ship_from_uf}
					onChange={(e) => set("ship_from_uf", e.target.value)}
					className={`${controlClass} h-10`}
				>
					{ufs.map((uf) => (
						<option key={uf} value={uf}>
							{uf}
						</option>
					))}
				</select>
				{errors.ship_from_uf && <p className="text-sm text-danger-600">{errors.ship_from_uf}</p>}
			</div>
		</>
	);
}
