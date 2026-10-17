import { Field as BaseField } from "@base-ui/react/field";
import type { ChangeEvent, InputHTMLAttributes, JSX } from "react";

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange"> {
	label: string;
	error?: string;
	onChange?: (event: ChangeEvent<HTMLInputElement>) => void;
}

// Field is a labelled input with its server-side validation error, built on
// Base UI Field so the label, control and error are wired for assistive tech.
export default function Field({ label, error, ...input }: Props): JSX.Element {
	return (
		<BaseField.Root invalid={Boolean(error)} className="flex flex-col gap-1">
			<BaseField.Label className="text-sm font-medium text-ink">{label}</BaseField.Label>
			<BaseField.Control
				{...input}
				className="h-10 rounded-md border border-line bg-card px-3 text-sm text-ink placeholder:text-muted focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
			/>
			<BaseField.Error match={true} className="text-sm text-danger-600">
				{error}
			</BaseField.Error>
		</BaseField.Root>
	);
}
