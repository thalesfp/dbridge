# DBBridge TUI Navigation Guide

## Interactive Form Controls

### Navigation Between Fields

| Key(s) | Action |
|--------|--------|
| `↓` or `Tab` | Move to next field |
| `↑` or `Shift+Tab` | Move to previous field |
| `Enter` | Confirm input and move to next group/submit |
| `Ctrl+C` | Cancel and exit form |

### SSL Mode Selection

When on the SSL Mode field:
- `↑` / `↓` - Navigate through options
- `Enter` - Select highlighted option

### Form Pages

The form has 3 pages:
1. **Profile Identification** (Step 1/3)
   - Profile Name
   - Database Name

2. **Connection Details** (Step 2/3)
   - Database Host
   - Database Port
   - Username

3. **Security & Performance** (Step 3/3)
   - SSL Mode (arrow-key selection)
   - Connection Pool Size
   - Password (masked with bullets, **optional** for trust/peer/cert auth)

After completing the main form:
- If you entered a password → Password confirmation prompt
- If you left password empty → Skips confirmation (passwordless authentication)

## Usage Modes

### Interactive Mode (Default)

Launches the rich TUI form with visual feedback:

```bash
# Fully interactive
dbridge config add

# Pre-fill profile name
dbridge config add production
```

**Features:**
- ✨ Arrow key navigation (↑/↓)
- ✅ Real-time validation
- 🎨 Styled interface
- 🔒 Password masking
- 📊 Progress tracking

### Flag Mode (For Scripts/Automation)

Use flags to skip the interactive form:

```bash
# With password in command (not recommended for security)
dbridge config add mydb \
  --host=localhost \
  --database=myapp \
  --username=admin \
  --password=secret

# Without password flag (will prompt securely)
dbridge config add mydb \
  --host=localhost \
  --database=myapp \
  --username=admin
# Enter password: [hidden input]
# Confirm password: [hidden input]

# With custom settings
dbridge config add production \
  --host=db.example.com \
  --port=5433 \
  --database=myapp_prod \
  --username=app_user \
  --ssl-mode=require \
  --pool-size=10
```

**When to use flag mode:**
- CI/CD pipelines
- Automated scripts
- Configuration management tools
- Non-TTY environments

## Clone and Edit Commands

### Clone Profile

```bash
# Interactive clone
dbridge config clone production staging

# Will open TUI form pre-filled with 'production' settings
# Name field will show 'staging'
# Can edit any field before saving
```

### Edit Profile

```bash
# Edit via the manage menu
dbridge --human config manage
# Select a profile → "Edit profile"

# Will open TUI form with current values
# Can modify any field including profile name
```

## Tips

1. **Navigation**: Use arrow keys (↑/↓) for the most intuitive navigation - no need for Shift+Tab!

2. **Validation**: The form validates in real-time. You'll see:
   - ✓ Green checkmark for valid input
   - ✗ Red error message for invalid input

3. **Required Fields**: All fields are required. The form won't let you skip required information.

4. **Password Security**:
   - In interactive mode: Password shown as bullets (••••)
   - In flag mode: Only prompted if --password flag not provided
   - Never include passwords in shell scripts (use prompting instead)
   - Password is **optional** - leave empty for passwordless authentication

5. **Passwordless Authentication**:
   PostgreSQL supports several authentication methods that don't require passwords:
   - **Trust**: No authentication required (local development)
   - **Peer**: Uses your OS username (Unix sockets only)
   - **Certificate**: Uses SSL client certificates
   - **GSSAPI/Kerberos**: Enterprise single sign-on
   - **LDAP**: Directory service authentication

   To use passwordless auth:
   - Interactive mode: Press Enter on the password field without typing
   - Flag mode: Omit the --password flag and press Enter at the prompt

6. **Defaults**:
   - Host: `localhost`
   - Port: `5432`
   - SSL Mode: `prefer`
   - Pool Size: `5`

## Keyboard Shortcuts Summary

```
Navigation:
  ↑/↓        Move between fields
  Tab        Next field
  Shift+Tab  Previous field
  Enter      Next group / Submit
  Ctrl+C     Cancel

Editing:
  Type       Enter text
  Backspace  Delete character
  Ctrl+U     Clear field (in some terminals)
```
