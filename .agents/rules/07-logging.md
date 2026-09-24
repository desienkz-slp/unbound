# Logging Rules

Use structured logs.

Do not put every DNS query into PostgreSQL synchronously.

Recommended:
- operational events -> structured service logs
- metrics -> Prometheus
- high-volume logs -> Loki
- sampled/query analytics -> optional specialized pipeline

Never log secrets, tokens, credentials, private keys, or unnecessary personal data.
