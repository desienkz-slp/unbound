# Password Reset Guide

Reset operator credentials via CLI directly on the server. **Zero-downtime** — no Unbound or API restart required.

## Method 1: `netora reset-password` (Recommended)

### Interactive Mode (password not stored in shell history)

```bash
netora reset-password
```

### Non-Interactive

```bash
netora reset-password admin "NewSecurePassword123"
```

## Method 2: `netora-reset-password` Script

```bash
netora-reset-password                              # interactive
netora-reset-password admin "NewSecurePassword123"  # direct
```

## Method 3: `netora-cli` Binary

```bash
# Reset password
netora-cli -mode reset-password -username admin -new-password "NewSecurePassword123"

# List operators
netora-cli -mode list-users

# Create new operator
netora-cli -mode create-user -username noc_operator -new-password "Pass123" -role OPERATOR
```

## Emergency Fallback (Direct PostgreSQL)

If binaries are unavailable:

```bash
python3 -c "
import bcrypt, subprocess
pw = b'NewSecurePassword123'
hashed = bcrypt.hashpw(pw, bcrypt.gensalt(10)).decode('utf-8')
subprocess.run(['sudo', '-u', 'postgres', 'psql', '-d', 'netora', '-c',
    f\"UPDATE users SET password_hash = '{hashed}', updated_at = NOW() WHERE username = 'admin';\"])
print('Password updated.')
"
```

> **Note:** All passwords are hashed with bcrypt (cost 10). Every credential change is recorded in the audit log.
