import type { JSX } from "react";

// ProductImage is a square, letterboxed product photo, or a neutral box
// with a parcel outline when the product has no image yet.
export default function ProductImage({
	src,
	alt,
	className = "",
	loading = "lazy",
}: {
	src: string;
	alt: string;
	className?: string;
	loading?: "lazy" | "eager";
}): JSX.Element {
	return (
		<div className={`flex aspect-square items-center justify-center overflow-hidden bg-white ${className}`}>
			{src ? (
				<img src={src} alt={alt} loading={loading} decoding="async" className="size-full object-contain" />
			) : (
				<svg
					viewBox="0 0 48 48"
					className="size-1/3 text-line"
					aria-hidden="true"
					fill="none"
					stroke="currentColor"
				>
					<path strokeWidth="2" d="M6 15 24 6l18 9v18l-18 9-18-9z M6 15l18 9 18-9 M24 24v18" />
				</svg>
			)}
		</div>
	);
}
