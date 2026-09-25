# Netora DNS — Autonomous CI/CD Architecture

## 1. Architectural Overview

Netora DNS uses an asynchronous, decentralized pull-based deployment model:

```
+-------------------------------------------------------------------------+
|                              GitHub                                     |
|  - Source of truth for code, Unbound templates, and policies            |
|  - Protected 'main' branch                                              |
|  - Fine-grained machine tokens / Deploy keys                            |
+------------------------------------+------------------------------------+
                                     |
                         1. Push / Pull Request
                                     v
+-------------------------------------------------------------------------+
|                       GitHub Actions (CI Runner)                        |
|  Job 1: ShellCheck & Secret Leak Prevention Audit                       |
|  Job 2: Go Unit Tests, Race Detector, & Binary Compilation              |
|  Job 3: Canonical Unbound Configuration Syntax & Security Guardrails    |
+------------------------------------+------------------------------------+
                                     |
                               2. Release
                                     v
+-------------------------------------------------------------------------+
|                  Independent Carrier DNS Nodes                          |
|                                                                         |
|   [ dns-dev ]             [ dns-jakarta-01 ]       [ dns-kediri-01 ]    |
|        |                          |                        |            |
|   Auto-Updater (Timer)      Auto-Updater (Timer)     Auto-Updater       |
|   - Polling / Webhook       - Polling / Webhook      - Polling          |
|   - Verification            - Verification           - Verification     |
|   - Local Staging           - Local Staging          - Local Staging    |
|   - Atomic Activation       - Atomic Activation      - Atomic Activation|
|   - Data-Plane Reload       - Data-Plane Reload      - Data-Plane Reload|
|   - Health Verification     - Health Verification    - Health Verif.    |
|   - Auto-Rollback on Error  - Auto-Rollback          - Auto-Rollback    |
+-------------------------------------------------------------------------+
```

## 2. Invariants & Security Principles

1. **Pull-Based Autonomy:** GitHub Actions does NOT hold persistent SSH root sessions to production resolvers. DNS nodes query GitHub autonomously.
2. **Data-Plane Isolation:** The DNS resolver (`unbound`) continues resolving customer queries even if GitHub is completely down, rate-limited, or inaccessible.
3. **No Direct Production Ingestion:** Code from GitHub is never blindly executed or copied directly to `/etc/unbound/`. Every candidate passes through a local staging sandbox and validation loop (`unbound-checkconf` + security guardrails).
4. **Zero Open Resolver:** Configuration validation strictly rejects any candidate containing `0.0.0.0/0 allow`.
5. **Zero Credential Leakage:** Machine tokens and SSH keys are stored on the server filesystem with `chmod 600` permissions and never committed to Git.
