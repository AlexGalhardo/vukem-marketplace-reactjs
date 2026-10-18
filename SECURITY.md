# Security Policy

## Supported versions

Only the latest release and `main` receive fixes.

## Reporting a vulnerability

Please report privately through [GitHub Security Advisories](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/security/advisories/new). Include steps to reproduce and the impact. Expect an answer within 7 days.

Do not test with real payment data: the app runs Stripe in test mode and AbacatePay in sandbox mode.

## Scope and practices

- OWASP Top 10:2025 checklist with the test that proves each control: [PLAN.md, section 5](PLAN.md#5-owasp-top-102025-checklist).
- Dependencies are pinned and scanned (`govulncheck`, Dependabot); DAST (ZAP baseline and nuclei) runs weekly.
- Issues in the vukem framework itself are reported upstream at <https://github.com/AlexGalhardo/vukem>.
