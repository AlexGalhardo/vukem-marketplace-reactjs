import type { Plugin } from "vite";

export default function vukem(options: { input: string[]; publicDir?: string; buildDir?: string }): Plugin;
