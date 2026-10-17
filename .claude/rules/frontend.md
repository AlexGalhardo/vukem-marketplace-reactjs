---
paths:
  - "resources/**"
  - "e2e/**"
---

# Frontend rules

- React 19 + TypeScript (strict) pages under `resources/js/pages`, rendered by Inertia v3. Props shapes live in `resources/js/types.ts` and mirror the Go controllers.
- Base UI (`@base-ui/react`) for behaviour of accessible primitives (menu, dialog, select, switch, tabs, toast); styling is Tailwind v4 only, tokens in `@theme` in `resources/css/app.css`. No shadcn, no other component kit.
- Inspired by a big LatAm marketplace layout (yellow header, blue accents) but never its brand: no logos, names or copied assets.
- Every string comes from `resources/lang/<locale>.json` through `useT()`; EN and PT-BR stay in sync (a test checks the key sets). Format money/dates with `Intl.*` and the active locale.
- Dark/light: `.dark` class on `<html>`, set before paint by the nonce'd inline script in `app.html`; toggle persists in `localStorage` (`theme`).
- Accessibility: visible labels, `aria-label` on icon-only buttons, visible focus, `prefers-reduced-motion` respected. No `dangerouslySetInnerHTML` with user content.
- Validate form input client-side with zod for UX only; the server (vukem `z`) is the source of truth.
- Biome (`bun run lint`) and `bun run typecheck` must pass. Load `frontend-design` before UI work and audit with `web-design-guidelines`.
