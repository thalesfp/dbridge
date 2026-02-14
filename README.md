# pgmcp - PostgreSQL MCP CLI

A cross-platform PostgreSQL CLI tool with MCP (Model Context Protocol) server support, designed for AI agents like Claude Code.

## Features

- ✅ **Secure Credential Storage**: Uses OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- ✅ **Multi-Profile Management**: Manage multiple database connections with named profiles
- ✅ **Ultra-Compact Output**: Token-optimized JSON format for AI agents (60-99% token reduction)
- ✅ **Smart Format Detection**: Automatically adapts output for humans (table) vs AI (JSON)
- ✅ **Read-Only Mode**: Safe query execution without risk of data modification
- 🚧 **MCP Server**: Coming soon - native integration with Claude Code and other AI tools
- 🔜 **Write Operations**: Planned for future release with safety controls and audit logging

## Installation

### From Source

```bash
git clone https://github.com/thalesgelinger/pgmcp.git
cd pgmcp
go build -o pgmcp ./cmd/pgmcp
./pgmcp --version
```

### Homebrew (Coming Soon)

```bash
brew tap thalesgelinger/tap
brew install pgmcp
```

## Quick Start

### 1. Add a Database Connection

```bash
pgmcp config add local \
  --host=localhost \
  --database=mydb \
  --username=admin
# Enter password when prompted
```

### 2. Execute Queries

```bash
# Simple count
pgmcp query "SELECT count(*) FROM users"
# Output: 1247

# List emails
pgmcp query "SELECT email FROM users LIMIT 3"
# Output: ["alice@example.com", "bob@example.com", "charlie@example.com"]

# Multi-column result
pgmcp query "SELECT id, name, active FROM users LIMIT 2"
# Output: {"cols":["id","name","active"],"rows":[[1,"Alice",true],[2,"Bob",false]]}
```

### 3. Manage Profiles

```bash
# List all profiles
pgmcp config list

# Show profile details
pgmcp config show local

# Remove profile
pgmcp config remove staging
```

## Output Formats

### Ultra-Compact JSON (for AI Agents)

The default format is optimized for token efficiency:

```bash
# Single value → Just the value
pgmcp query "SELECT count(*) FROM users"
1247  # 5 tokens (vs 450 with verbose JSON)

# Single column → Array
pgmcp query "SELECT email FROM users LIMIT 3"
["alice@example.com","bob@example.com","charlie@example.com"]  # 30 tokens

# Multi-column → Compact array format
pgmcp query "SELECT id, name FROM users LIMIT 2"
{"cols":["id","name"],"rows":[[1,"Alice"],[2,"Bob"]]}  # 180 tokens
```

### Format Options

```bash
# Explicit compact format
pgmcp query "..." --format=compact

# Human-readable table (auto-detected in terminal)
pgmcp query "..."

# Force specific format
pgmcp query "..." --format=table
```

## Configuration

Configuration file: `~/.config/pgmcp/config.yaml`

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

## Project Status

**Phase 1: Core CLI** ✅ **COMPLETED**
- [x] Basic project structure
- [x] Credential storage (OS keychain)
- [x] Configuration management
- [x] Profile management
- [x] Query execution
- [x] Ultra-compact output format

**Phase 2: Advanced Features** 🚧 **IN PROGRESS**
- [ ] Schema inspection commands (list-schemas, list-tables, describe-table)
- [ ] Interactive setup wizard
- [ ] Table output formatter
- [ ] Enhanced error messages

**Phase 2.5: Write Operations** 🔜 **PLANNED FOR LATER**
- [ ] Write operations (INSERT, UPDATE, DELETE)
- [ ] Safety checks and confirmations
- [ ] Audit logging
- [ ] Dry-run mode

**Phase 3: MCP Server** 🔜 **PLANNED**
- [ ] MCP server implementation
- [ ] JSON-RPC 2.0 protocol
- [ ] Claude Desktop integration
- [ ] MCP tools (execute_sql, list_tables, etc.)

## Development

### Build

```bash
go build -o pgmcp ./cmd/pgmcp
```

### Run Tests

```bash
go test ./...
```

### Dependencies

- Go 1.23+
- PostgreSQL 12+

## Architecture

```
pgmcp/
├── cmd/
│   └── pgmcp/              # Main CLI binary
├── internal/
│   ├── cli/                # CLI commands
│   ├── config/             # Configuration management
│   ├── credentials/        # OS keychain integration
│   ├── database/           # PostgreSQL client
│   └── output/             # Output formatters
└── test/                   # Tests
```

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please open an issue or PR.

## Security

Credentials are stored in OS-native secure storage:
- **macOS**: Keychain
- **Windows**: Credential Manager
- **Linux**: Secret Service (fallback: encrypted file)

Passwords are NEVER stored in config files or logs.
