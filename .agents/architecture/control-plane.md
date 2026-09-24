# Control Plane

Recommended services:

netora-api
- authentication
- RBAC
- CRUD
- audit
- config versions

netora-controller
- desired-state reconciliation
- config rendering
- validation
- apply
- rollback
- health

netora-worker
- filter downloads
- scheduled jobs
- health checks
- maintenance

netora-web
- React/TypeScript UI

PostgreSQL:
- users/roles
- nodes
- interfaces
- networks
- policies
- filters
- upstreams
- config versions
- audit records

The control plane must tolerate temporary database/API failure without taking DNS down.
