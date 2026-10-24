import type { JSX, ReactNode } from "react";

// Own line icons: 24px grid, 1.75 stroke, round joins, currentColor. Always
// decorative (aria-hidden); the control around them carries the name.
function Icon({ children, className = "size-5" }: { children: ReactNode; className?: string }): JSX.Element {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.75"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			className={className}
		>
			{children}
		</svg>
	);
}

type P = { className?: string };

export const SearchIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<circle cx="10.5" cy="10.5" r="6.5" />
		<path d="m15.5 15.5 5 5" />
	</Icon>
);

export const CartIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M3 4h2.2l2.3 11h10.8l2.2-8H6.4" />
		<circle cx="9.5" cy="19.5" r="1.3" />
		<circle cx="17" cy="19.5" r="1.3" />
	</Icon>
);

export const PinIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M12 21s-6.5-6.1-6.5-11a6.5 6.5 0 0 1 13 0c0 4.9-6.5 11-6.5 11z" />
		<circle cx="12" cy="10" r="2.3" />
	</Icon>
);

export const ChevronLeftIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="m14.5 5.5-6.5 6.5 6.5 6.5" />
	</Icon>
);

export const ChevronRightIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="m9.5 5.5 6.5 6.5-6.5 6.5" />
	</Icon>
);

export const ChevronDownIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="m6 9.5 6 6 6-6" />
	</Icon>
);

export const TruckIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M2.5 6.5h11v9h-11zM13.5 9.5h4l3 3v3h-7" />
		<circle cx="6.5" cy="17.5" r="1.8" />
		<circle cx="17" cy="17.5" r="1.8" />
	</Icon>
);

export const ShieldIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M12 3 4.5 6v5.5c0 4.4 3.2 8.1 7.5 9.5 4.3-1.4 7.5-5.1 7.5-9.5V6z" />
		<path d="m8.8 12 2.2 2.2 4.2-4.4" />
	</Icon>
);

export const UserIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<circle cx="12" cy="8.5" r="3.8" />
		<path d="M4.5 20c1.3-3.6 4.2-5.4 7.5-5.4s6.2 1.8 7.5 5.4" />
	</Icon>
);

export const CardIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<rect x="2.8" y="5.5" width="18.4" height="13" rx="2" />
		<path d="M2.8 9.8h18.4M6.5 15h3.5" />
	</Icon>
);

export const TagIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M3.5 12.2V4.5a1 1 0 0 1 1-1h7.7l8.3 8.3a1 1 0 0 1 0 1.4l-7.3 7.3a1 1 0 0 1-1.4 0z" />
		<circle cx="8" cy="8" r="1.4" />
	</Icon>
);

export const StoreIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M4 9.5V20h16V9.5M3 9.5 5 4h14l2 5.5c0 1.5-1.2 2.5-2.6 2.5S16 11 16 9.5c0 1.5-1.4 2.5-3 2.5h-2c-1.6 0-3-1-3-2.5C8 11 6.9 12 5.6 12 4.2 12 3 11 3 9.5z" />
		<path d="M9.5 20v-5h5v5" />
	</Icon>
);

export const GridIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<rect x="4" y="4" width="6.5" height="6.5" rx="1" />
		<rect x="13.5" y="4" width="6.5" height="6.5" rx="1" />
		<rect x="4" y="13.5" width="6.5" height="6.5" rx="1" />
		<rect x="13.5" y="13.5" width="6.5" height="6.5" rx="1" />
	</Icon>
);

export const ListIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<rect x="4" y="4.5" width="5" height="5" rx="1" />
		<rect x="4" y="14.5" width="5" height="5" rx="1" />
		<path d="M12 6h8M12 8.5h5M12 16h8M12 18.5h5" />
	</Icon>
);

export const FilterIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M4 6h16M7 12h10M10 18h4" />
	</Icon>
);

export const PauseIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M9 6v12M15 6v12" />
	</Icon>
);

export const PlayIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M8 5.5v13l10-6.5z" />
	</Icon>
);

export const ChatIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M4 5.5h16v10H10l-4.5 3.5v-3.5H4z" />
		<path d="M8.5 10.5h7" />
	</Icon>
);

export const BoxIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M3.5 7.5 12 3.5l8.5 4v9L12 20.5l-8.5-4z" />
		<path d="M3.5 7.5 12 11.5l8.5-4M12 11.5v9" />
	</Icon>
);

// Category glyphs, keyed by the seeded category slugs; unknown slugs get
// the tag glyph.
const categoryPaths: Record<string, ReactNode> = {
	electronics: (
		<>
			<rect x="3" y="5" width="18" height="11.5" rx="1.5" />
			<path d="M8.5 20h7M12 16.5V20" />
		</>
	),
	phones: (
		<>
			<rect x="7" y="2.8" width="10" height="18.4" rx="2" />
			<path d="M11 18h2" />
		</>
	),
	computers: (
		<>
			<rect x="4.5" y="5" width="15" height="10" rx="1.2" />
			<path d="M2.5 18.5h19l-1.5-3.5H4z" />
		</>
	),
	home: (
		<>
			<path d="M3.5 11 12 4l8.5 7" />
			<path d="M5.5 9.5V20h13V9.5M10 20v-5.5h4V20" />
		</>
	),
	appliances: (
		<>
			<rect x="5" y="3" width="14" height="18" rx="1.5" />
			<circle cx="12" cy="13" r="4" />
			<path d="M8 6.2h1.5" />
		</>
	),
	fashion: (
		<>
			<path d="M9 3.5 4 6l1.8 4 2.2-1V20.5h8V9l2.2 1L20 6l-5-2.5c-.5 1.4-1.6 2.2-3 2.2s-2.5-.8-3-2.2z" />
		</>
	),
	sports: (
		<>
			<circle cx="12" cy="12" r="8.5" />
			<path d="M3.8 9.5c3.5.3 6.6 2.4 8.2 5.6M20.2 14.5c-3.5-.3-6.6-2.4-8.2-5.6M9 3.9c-.3 3.4 1.3 6.6 3 8.1M15 20.1c.3-3.4-1.3-6.6-3-8.1" />
		</>
	),
	beauty: (
		<>
			<rect x="9" y="10" width="6" height="11" rx="1" />
			<path d="M10 10V6.5l4-3V10" />
		</>
	),
	toys: (
		<>
			<circle cx="7.5" cy="6.5" r="2.5" />
			<circle cx="16.5" cy="6.5" r="2.5" />
			<circle cx="12" cy="13.5" r="6.5" />
			<path d="M10 12.5h.01M14 12.5h.01M10.2 16c1.1.8 2.5.8 3.6 0" />
		</>
	),
	tools: (
		<>
			<path d="M14.5 6.2a4 4 0 0 0-5.4 4.9L3.5 16.7l3.8 3.8 5.6-5.6a4 4 0 0 0 4.9-5.4l-2.4 2.4-2.6-.7-.7-2.6z" />
		</>
	),
	books: (
		<>
			<path d="M4 5.5c2.8-1 5.5-.8 8 .8 2.5-1.6 5.2-1.8 8-.8V19c-2.8-1-5.5-.8-8 .8-2.5-1.6-5.2-1.8-8-.8z" />
			<path d="M12 6.3v13.5" />
		</>
	),
	"car-parts": (
		<>
			<path d="M4 15.5V12l2-4.5h12l2 4.5v3.5z" />
			<path d="M4 12h16" />
			<circle cx="7.5" cy="17" r="1.6" />
			<circle cx="16.5" cy="17" r="1.6" />
		</>
	),
};

export function CategoryIcon({ slug, className = "size-7" }: { slug: string; className?: string }): JSX.Element {
	return (
		<Icon className={className}>
			{categoryPaths[slug] ?? (
				<>
					<path d="M3.5 12.2V4.5a1 1 0 0 1 1-1h7.7l8.3 8.3a1 1 0 0 1 0 1.4l-7.3 7.3a1 1 0 0 1-1.4 0z" />
					<circle cx="8" cy="8" r="1.4" />
				</>
			)}
		</Icon>
	);
}

export const EyeIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z" />
		<circle cx="12" cy="12" r="3" />
	</Icon>
);

export const EyeOffIcon = (p: P): JSX.Element => (
	<Icon {...p}>
		<path d="M3 3l18 18M10.6 5.1A10.5 10.5 0 0 1 12 5c6.4 0 10 7 10 7a17 17 0 0 1-3.1 3.9M6.5 6.5A16.6 16.6 0 0 0 2 12s3.6 7 10 7a9.7 9.7 0 0 0 4.3-1M9.9 9.9a3 3 0 0 0 4.2 4.2" />
	</Icon>
);
