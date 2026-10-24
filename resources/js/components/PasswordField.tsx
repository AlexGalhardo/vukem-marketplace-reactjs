import { Field as BaseField } from "@base-ui/react/field";
import { type ChangeEvent, type InputHTMLAttributes, type JSX, useState } from "react";
import { PASSWORD_MAX, PASSWORD_RULES, passwordChecks } from "../lib/credentials";
import { useT } from "../lib/i18n";
import { EyeIcon, EyeOffIcon } from "./icons";

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "type" | "value"> {
	label: string;
	value: string;
	error?: string;
	// requirements shows the live checklist of the password rules.
	requirements?: boolean;
	onChange: (event: ChangeEvent<HTMLInputElement>) => void;
}

// PasswordField is a password input with a show/hide toggle and, for new
// passwords, a live checklist of the rules the server enforces.
export default function PasswordField({ label, error, requirements = false, value, ...input }: Props): JSX.Element {
	const { t } = useT();
	const [visible, setVisible] = useState(false);
	const checks = passwordChecks(value);

	return (
		<BaseField.Root invalid={Boolean(error)} className="flex flex-col gap-1">
			<BaseField.Label className="text-sm font-medium text-ink">{label}</BaseField.Label>
			<div className="relative">
				<BaseField.Control
					maxLength={PASSWORD_MAX}
					{...input}
					value={value}
					type={visible ? "text" : "password"}
					className="h-10 w-full rounded-md border border-line bg-card pr-10 pl-3 text-sm text-ink placeholder:text-muted focus:border-action-500 focus:outline-none data-invalid:border-danger-600"
				/>
				<button
					type="button"
					onClick={() => setVisible((v) => !v)}
					aria-pressed={visible}
					aria-label={t(visible ? "account.password.hide" : "account.password.show")}
					className="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-md text-muted hover:text-ink"
				>
					{visible ? <EyeOffIcon className="size-5" /> : <EyeIcon className="size-5" />}
				</button>
			</div>
			{requirements && (
				<ul aria-live="polite" className="mt-1 space-y-0.5 text-sm">
					{PASSWORD_RULES.map((rule) => (
						<li key={rule} className={checks[rule] ? "text-success-600" : "text-muted"}>
							<span aria-hidden="true">{checks[rule] ? "✓" : "○"}</span>{" "}
							<span className="sr-only">
								{t(checks[rule] ? "account.password.met" : "account.password.unmet")}{" "}
							</span>
							{t(`account.password.rule_${rule}`)}
						</li>
					))}
				</ul>
			)}
			<BaseField.Error match={true} className="text-sm text-danger-600">
				{error}
			</BaseField.Error>
		</BaseField.Root>
	);
}
