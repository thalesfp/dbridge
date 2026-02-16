# Passwordless Authentication Support

DBBridge now supports PostgreSQL connections without passwords, enabling various authentication methods commonly used in development and enterprise environments.

## Supported Authentication Methods

### 1. **Trust Authentication**
No authentication required - useful for local development.

**PostgreSQL Configuration** (`pg_hba.conf`):
```conf
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all             all                                     trust
host    all             all             127.0.0.1/32           trust
```

**DBBridge Profile**:
```bash
# Interactive mode - just press Enter on password field
dbridge config add local

# Flag mode - omit --password
dbridge config add local --database=myapp --username=postgres
# Press Enter when prompted for password (leave empty)
```

### 2. **Peer Authentication**
Uses your OS username for authentication over Unix domain sockets.

**PostgreSQL Configuration** (`pg_hba.conf`):
```conf
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all             all                                     peer
```

**Requirements**:
- Unix/Linux/macOS only
- Database username must match OS username
- Connection via Unix socket only

**DBBridge Profile**:
```bash
# If your OS username is 'john', create profile with same username
dbridge config add local --database=myapp --username=john
# Leave password empty
```

### 3. **Certificate Authentication**
Uses SSL client certificates for authentication.

**PostgreSQL Configuration** (`pg_hba.conf`):
```conf
# TYPE  DATABASE        USER            ADDRESS                 METHOD
hostssl all             all             0.0.0.0/0              cert
```

**DBBridge Profile**:
```bash
dbridge config add secure \
  --database=myapp \
  --username=certuser \
  --ssl-mode=require
# Leave password empty
# Note: Certificate files must be configured in PostgreSQL connection
```

### 4. **GSSAPI/Kerberos (Enterprise SSO)**
Uses Kerberos tickets for authentication.

**PostgreSQL Configuration** (`pg_hba.conf`):
```conf
# TYPE  DATABASE        USER            ADDRESS                 METHOD
host    all             all             0.0.0.0/0              gss
```

**DBBridge Profile**:
```bash
# Requires valid Kerberos ticket (kinit)
dbridge config add enterprise \
  --database=myapp \
  --username=user@REALM.COM
# Leave password empty
```

## Usage Examples

### Interactive Mode

**With Password** (traditional):
```bash
$ dbridge config add mydb
📋 Profile Identification (Step 1/3)
Profile Name: mydb
Database Name: myapp

🔌 Connection Details (Step 2/3)
Host: localhost
Port: 5432
Username: admin

🔐 Security & Performance (Step 3/3)
SSL Mode: prefer
Pool Size: 5
Password: ••••••••

Confirm Password: ••••••••

✓ Profile 'mydb' added successfully!
  Host: localhost:5432
  Database: myapp
  Authentication: macOS Keychain
```

**Without Password** (passwordless):
```bash
$ dbridge config add localdev
📋 Profile Identification (Step 1/3)
Profile Name: localdev
Database Name: myapp

🔌 Connection Details (Step 2/3)
Host: localhost
Port: 5432
Username: postgres

🔐 Security & Performance (Step 3/3)
SSL Mode: prefer
Pool Size: 5
Password: [press Enter without typing]

✓ Profile 'localdev' added successfully!
  Host: localhost:5432
  Database: myapp
  Authentication: Passwordless (trust/peer/cert)
```

### Flag Mode

**With Password Prompt**:
```bash
$ dbridge config add mydb --database=myapp --username=admin
Enter password for admin@localhost (or press Enter for passwordless auth): ••••••••
Confirm password: ••••••••
✓ Profile 'mydb' added successfully
✓ Credentials stored in macOS Keychain
```

**Passwordless** (press Enter at password prompt):
```bash
$ dbridge config add localdev --database=myapp --username=postgres
Enter password for postgres@localhost (or press Enter for passwordless auth): [press Enter]
✓ Profile 'localdev' added successfully
✓ Using passwordless authentication
```

**Passwordless** (with explicit empty password flag):
```bash
$ dbridge config add localdev --database=myapp --username=postgres --password=""
✓ Profile 'localdev' added successfully
✓ Using passwordless authentication
```

## Common Use Cases

### Local Development with Docker

When running PostgreSQL in Docker with trust authentication:

```bash
# Docker compose with trust auth
# docker-compose.yml:
# environment:
#   POSTGRES_HOST_AUTH_METHOD: trust

# Create passwordless profile
dbridge config add docker --database=myapp --username=postgres
```

### Development with Peer Authentication

On macOS/Linux with peer authentication:

```bash
# PostgreSQL configured with peer auth
# Your OS username: john

# Create profile matching OS username
dbridge config add local --database=myapp --username=john
```

### Production with Certificate Authentication

For production environments using client certificates:

```bash
# Create profile with SSL required, no password
dbridge config add production \
  --host=db.example.com \
  --database=myapp_prod \
  --username=app_user \
  --ssl-mode=require \
  --password=""
```

## Converting Between Password and Passwordless

### Add Password to Passwordless Profile

```bash
# Edit the profile via the manage menu
dbridge --human config manage
# Select 'localdev' → "Edit profile"

# In the form, enter a password
# The profile will now use password authentication
```

### Remove Password from Password-Protected Profile

```bash
# Edit the profile via the manage menu
dbridge --human config manage
# Select 'mydb' → "Edit profile"

# In the form, delete the password (leave field empty)
# The profile will now use passwordless authentication
```

## Troubleshooting

### "Password authentication failed"

If you created a passwordless profile but get authentication errors:

1. Check PostgreSQL's `pg_hba.conf` allows your authentication method
2. Verify the authentication method matches your connection:
   - Trust: Check IP/hostname matches
   - Peer: Verify Unix socket connection and username matches
   - Certificate: Ensure SSL client cert is configured
   - GSSAPI: Check Kerberos ticket is valid (`klist`)

3. Check PostgreSQL logs for authentication details:
   ```bash
   # On macOS with Homebrew
   tail -f /usr/local/var/log/postgres.log

   # On Linux
   sudo tail -f /var/log/postgresql/postgresql-15-main.log
   ```

### "No password provided but required"

Your PostgreSQL server expects a password. Either:

1. Configure PostgreSQL for passwordless auth (see above), or
2. Edit the profile and add a password:
   ```bash
   dbridge --human config manage
   # Select 'myprofile' → "Edit profile"
   ```

## Security Considerations

**Trust Authentication**:
- ⚠️ No security - anyone can connect
- ✅ Acceptable for local development
- ❌ Never use in production or on public networks

**Peer Authentication**:
- ✅ Secure for local connections
- ✅ Good for development and single-user systems
- ⚠️ Only works over Unix sockets

**Certificate Authentication**:
- ✅ Very secure with proper certificate management
- ✅ Suitable for production
- ⚠️ Requires certificate infrastructure

**GSSAPI/Kerberos**:
- ✅ Enterprise-grade security
- ✅ Single sign-on capability
- ⚠️ Complex setup and maintenance

## Best Practices

1. **Development**: Use trust or peer authentication for simplicity
2. **Staging**: Use password authentication with strong passwords
3. **Production**: Use certificate or GSSAPI authentication
4. **Local Docker**: Trust authentication is fine for isolated environments
5. **Shared Servers**: Always require authentication (password/cert/GSSAPI)

## Related Commands

```bash
# View profile details (shows authentication type)
dbridge config show myprofile

# List all profiles
dbridge config list

# Clone a passwordless profile
dbridge config clone localdev staging

# Edit authentication settings via manage menu
dbridge --human config manage
```
