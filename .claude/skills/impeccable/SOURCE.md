# Source

- Upstream: https://github.com/pbakaus/impeccable (site: https://impeccable.style/), license Apache 2.0.
- Version: release `cli-v4.6.0` (skill `version: 4.6.0`, `scripts/VERSION` 0.1.15), commit `8a0e198da34ebc5b936a9d5975fb7a3b1cf353b7`.
- Installed project-scoped by copying `.claude/skills/impeccable/` from the tagged tarball
  (`gh api repos/pbakaus/impeccable/tarball/8a0e198…`), not with `npx skills add` / `npx impeccable install`,
  which can write user-level config.
- The `scripts/impeccable` launcher downloads an engine binary into `~/.impeccable` on first run (outside the
  repo); it is not run here. Use the SKILL.md "Launcher unavailable" path (read PRODUCT.md / DESIGN.md directly),
  or set `IMPECCABLE_HOME` to a local cache if you opt in.
- Update: re-copy the folder from a newer stable tag and bump this file.
