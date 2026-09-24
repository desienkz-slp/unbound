# Skill: Unbound Engineering

Use this skill for:
- unbound.conf generation
- interfaces and access-control
- views
- cache tuning
- prefetch
- serve-expired
- DNSSEC
- local-zone/local-data
- unbound-control
- configuration validation
- runtime diagnostics

Rules:
1. Check the installed Unbound version before relying on a feature.
2. Validate generated configuration with `unbound-checkconf`.
3. Prefer reload over restart when safe.
4. Keep control socket private.
5. Test IPv4 and IPv6 independently.
6. Measure rather than assume performance.
7. Preserve last-known-good configuration.

Deliverables:
- generated config
- validation result
- tests
- operational notes
