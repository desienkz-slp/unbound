# Local Configuration Preservation Rules (Anti-Overwrite Policy)

Never overwrite, reset, or alter running local settings during updates.

## Core Rules:
1. **Preserve Running Local State:**
   - When deploying updates (code changes, UI updates, binary builds, migrations), all existing runtime configurations on active nodes (`netora-dns1`, `netora-dns2`, etc.) MUST remain intact.
   - Never overwrite active configurations in PostgreSQL (`nodes.config_json`, `listeners`, `upstreams`, `client_networks`, `filters`, `users`) with default/hardcoded values.

2. **Protected Entities:**
   - User credentials & password hashes (never reset admin password during update).
   - Upstream server selections and custom forwarders.
   - Network listener bindings and custom IP assignments.
   - Client access-control subnets and ACL lists.
   - Custom blacklists, whitelists, safe search, and parental control toggles.
   - Active performance tuning parameters.

3. **Additive-Only / Non-Destructive Schema & Config Evolution:**
   - If an update introduces new configuration fields, use additive defaults: only populate new fields if they are missing or unset.
   - Existing values must NEVER be replaced or reverted to defaults without explicit instruction from the user.

4. **Installer and Deploy Script Discipline:**
   - Deployment scripts (SCP, rsync, git pull) must target only application binaries (`/usr/local/bin/netora-api`) and static web assets (`/var/www/netora` or `deploy/ui/`).
   - Do NOT run destructive initialization scripts or default seeders against live databases.
   - Always verify that the running node configuration is preserved before and after any deployment.
