# Cyber Defense Center

## Scope (Defensive-Only)
Cyber Defense Center provides defensive cybersecurity analysis only.

It supports:
- security audits
- risk assessment
- incident response planning
- compliance checklists
- asset review
- policy review

It does **not** provide:
- exploit automation
- unauthorized access
- credential theft
- malware or persistence instructions
- destructive/autonomous shutdown actions
- live scanning

## Modes
- `security_audit`
- `risk_assessment`
- `incident_response`
- `compliance_checklist`
- `asset_review`
- `policy_review`

## API
- `POST /api/cyber/analyze`
- `GET /api/cyber/analyses` (latest 20 by current user)

## Model Environment Variables
- `TOGETHER_MODEL_SECURITY`
- fallback: `TOGETHER_MODEL_REASONING` → `TOGETHER_MODEL_COMPLEX` → `TOGETHER_MODEL`
- `TOGETHER_SECURITY_TIMEOUT_SECONDS` (default `120`)
- `TOGETHER_SECURITY_MAX_TOKENS` (default `2500`)

## Credit Behavior
- Normal user: charge 1 credit on successful analysis.
- Owner/admin: no charge.
- Failed provider call: no charge.
- Credit event reason: `cyber_analysis`.

## Human Approval Requirement
- Human review is always required for sensitive remediation ideas.
- The module explicitly blocks offensive and autonomous actions.

## Future Expansion
- asset inventory
- audit log expansion
- approval workflows
- smart glasses / hybrid security shield as final future phase
