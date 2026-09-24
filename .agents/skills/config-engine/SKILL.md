# Skill: Configuration Engine

Implement desired-state reconciliation.

Inputs:
- database desired state
- node capabilities
- deployment profile

Outputs:
- generated Unbound configuration
- generated auxiliary files
- validation report

Required:
- deterministic generation
- stable ordering
- atomic writes
- versioning
- diff generation
- rollback
- health verification
