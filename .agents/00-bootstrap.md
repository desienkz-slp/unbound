# AI Agent Bootstrap

Read this file before doing any implementation.

## Execution order

1. Read `AGENTS.md`.
2. Read every file in `.agents/rules/`.
3. Read every relevant skill in `.agents/skills/`.
4. Read `.agents/architecture/target.md`.
5. Read `.agents/architecture/data-plane.md`.
6. Read `.agents/architecture/control-plane.md`.
7. Read `.agents/architecture/security.md`.
8. Read `.agents/architecture/observability.md`.
9. Read `.agents/mcp/README.md`.
10. Inspect the repository.
11. Produce an implementation plan before making large changes.
12. Implement in phases.
13. Run tests after each phase.
14. Never claim completion without evidence.

## First response/task behavior

Before coding, report:
- repository state
- detected stack
- missing dependencies
- current architecture
- risks
- proposed phase
- exact files you intend to create/change

Then begin Phase 0 unless the user explicitly requests another phase.

## Agent autonomy

The agent may create files, install project-local dependencies, run tests, generate configs, and create development environments.

The agent MUST NOT:
- expose secrets
- disable security controls merely to make tests pass
- bind DNS to all public interfaces without an explicit security decision
- delete user data/configuration without backup and confirmation
- silently replace existing production configuration

## MCP rule

MCP is an optional control/integration layer. It must never be a runtime dependency of Unbound.
