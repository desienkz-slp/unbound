# Skill: DNS Policy Engine

Model policy as ordered, explicit rules.

Recommended precedence:
1. emergency deny/allow override
2. explicit allowlist
3. explicit custom rules
4. category policy
5. blocklist feeds
6. normal resolution

Support:
- exact domains
- subdomains
- wildcard semantics where supported
- local records
- redirect policy
- NXDOMAIN/REFUSED/static behavior
- scheduled policies

Every block decision should be explainable.
