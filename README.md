# dbbridge - Database Bridge for AI Agents

A cross-platform database CLI tool with MCP (Model Context Protocol) server support, designed to bridge AI agents like Claude Code to databases.

## Features

- **Multi-Database Support**: PostgreSQL (more databases planned)
- **Secure Credential Storage**: Uses OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service) — passwords never stored in config files or logs
- **Multi-Profile Management**: Manage multiple database connections with named profiles
- **Multiple Output Formats**: Compact JSON (token-efficient for AI agents), table, and CSV
- **Schema Inspection**: List schemas, tables, and describe table structures
- **Read-Only Mode**: Safe query execution without risk of data modification

## Installation

### From Source

```bash
git clone https://github.com/thalesgelinger/dbbridge.git
cd dbbridge
make build
./bin/dbbridge --version
```

### Make Commands

```bash
make build          # Build binary
make test           # Run tests
make test-coverage  # Generate coverage report
make db-setup       # Setup test database
make clean          # Remove build artifacts
make help           # Show all commands
```

### Dependencies

- Go 1.25+
- PostgreSQL 12+ (for PostgreSQL connections)

## Quick Start

### 1. Add a Database Connection

```bash
dbbridge config add local \
  --host=localhost \
  --database=mydb \
  --username=admin
# Enter password when prompted
```

### 2. Execute Queries

```bash
# Simple count
dbbridge query "SELECT count(*) FROM users"
# Output: 1247

# List emails
dbbridge query "SELECT email FROM users LIMIT 3"
# Output: ["alice@example.com", "bob@example.com", "charlie@example.com"]

# Multi-column result
dbbridge query "SELECT id, name, active FROM users LIMIT 2"
# Output: {"cols":["id","name","active"],"rows":[[1,"Alice",true],[2,"Bob",false]]}
```

### 3. Manage Profiles

```bash
# List all profiles
dbbridge config list

# Show profile details
dbbridge config show local

# Remove profile
dbbridge config remove staging
```

## Configuration

Configuration file: `~/.config/dbbridge/config.yaml`

```yaml
settings:
  default_profile: "local"
  output:
    default: "auto"          # auto, compact, table, csv
    auto_detect_tty: true    # Auto-switch based on TTY
    include_types: true      # Include column types
    include_timing: false    # Exclude timing (save tokens)
    smart_simplify: true     # Smart format detection

profiles:
  local:
    host: "localhost"
    port: 5432
    database: "mydb"
    username: "admin"
    ssl_mode: "prefer"
    readonly: false
    # Password stored in OS Keychain
```

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please open an issue or PR.
