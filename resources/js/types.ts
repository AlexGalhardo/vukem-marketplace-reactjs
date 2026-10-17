// Shapes of the props the Go controllers send.

export interface User {
	id: number;
	name: string;
	email: string;
	email_verified_at: string | null;
	role: "user" | "admin";
}

export interface SharedProps {
	appName: string;
	locale: string;
	locales: string[];
	errors: Record<string, string>;
	// Present once the Auth phase ships.
	auth?: { user: User | null };
	[key: string]: unknown;
}

// Account/Security page props (controllers/account.ShowSecurity).
export interface SecurityProps {
	twoFactorEnabled: boolean;
	recoveryCodesLeft: number;
}
