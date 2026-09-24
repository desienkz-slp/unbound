# Architecture Rules

- Separate data plane from control plane.
- Unbound must remain independently operable.
- UI communicates with API, never edits Unbound files directly.
- API writes desired state; generator renders runtime state.
- Runtime configuration is generated from versioned desired state.
- Use atomic apply: render -> validate -> stage -> health-check -> activate.
- Keep last-known-good configuration.
- Prefer horizontal scaling of DNS nodes over making one node infinitely large.
- Design all components to be stateless where possible except PostgreSQL and explicit node-local runtime state.
- Treat multi-IP/multi-interface as a core capability.
