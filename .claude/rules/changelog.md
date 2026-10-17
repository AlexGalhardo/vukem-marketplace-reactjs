---
paths:
  - "CHANGELOG.md"
---

# Changelog rules

- Keep a Changelog 1.1.0, SemVer tags `vX.Y.Z`.
- Every relevant change set adds a line under `[Unreleased]` (Added, Changed, Deprecated, Removed, Fixed, Security).
- Releasing: move `[Unreleased]` entries into `## [X.Y.Z] - YYYY-MM-DD`, update the compare links, tag `vX.Y.Z`, push the tag; `release.yml` publishes the GitHub release.
- Never edit a released section except to fix a typo; never move a pushed tag.
