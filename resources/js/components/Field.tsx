import { Field as BaseField } from "@base-ui/react/field";
import type { ChangeEvent, InputHTMLAttributes, JSX } from "react";

// DEFAULT_MAX_LENGTH bounds every text input that sets no limit of its own;
// required inputs already have a minimum of one character.
const DEFAULT_MAX_LENGTH = 255;

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange"> {
	label: string;
	error?: string;
	// hint is shown under the control and announced with it.
	hint?: string;
	onChange?: (event: ChangeEvent<HTMLInputElement>) => void;
}

// Field is a labelled input with its server-side validation error, built on
// Base UI Field so the label, control and error are wired for assistive tech.
export default function Field({ label, error, hint, ...input }: Props): JSX.Element {
	return (
		<BaseField.Root invalid={Boolean(error)} className="flex flex-col gap-1">
			<BaseField.Label className="text-sm font-medium text-ink">{label}</BaseField.Label>
			<BaseField.Control
				maxLength={DEFAULT_MAX_LENGTH}
				{...input}
				className="h-10 rounded-md border border-line bg-card px-3 text-sm text-ink placeholder:text-muted focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
			/>
			{hint && <BaseField.Description className="text-sm text-muted">{hint}</BaseField.Description>}
			<BaseField.Error match={true} className="text-sm text-danger-600">
				{error}
			</BaseField.Error>
		</BaseField.Root>
	);
}
