# Koschei command-center visual translation — 2026-09-26

This change translates the approved Koschei visual concept into the production public homepage and authenticated customer dashboard.

## Product intent

- Public homepage: communicate the full Koschei security-intelligence surface instead of reading like a login page.
- Customer dashboard: present existing account-backed evidence, investigations, monitoring and ARVIS relationships as a large mission-control surface.
- Preserve evidence semantics: no synthetic telemetry, invented risk scores or fabricated account state.
- Keep Solana identified as the live core while other supported paths retain their existing evidence boundaries.
- Keep mobile navigation responsive and off-canvas rather than squeezing the desktop command center into the viewport.

## Implementation

- Added `public/css/koschei-command-vision.css` as a presentation-only override layer.
- Reworked the homepage hero visual into a conceptual ARVIS intelligence radar with explicit non-live labeling.
- Reframed the authenticated workspace as `Command Center`, `ARVIS Radar`, `Live Evidence`, and `Command Modules`.
- Reused existing real workspace KPIs, account evidence, investigation history and alert sources instead of introducing mock data.
