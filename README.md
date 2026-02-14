# dbbridge - Database Bridge for AI Agents

A cross-platform database CLI tool with MCP (Model Context Protocol) server support, designed to bridge AI agents like Claude Code to databases.

## Features

- ✅ **Multi-Database Support**: PostgreSQL (more databases coming soon)
- ✅ **Secure Credential Storage**: Uses OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- ✅ **Multi-Profile Management**: Manage multiple database connections with named profiles
- ✅ **Multiple Output Formats**:
  - **Compact JSON**: Ultra token-efficient for AI agents (60-99% reduction)
  - **Table**: Beautiful Unicode borders for human readability
  - **CSV**: Standard CSV export with headers
- ✅ **Schema Inspection**: List schemas, tables, and describe table structures
- ✅ **Read-Only Mode**: Safe query execution without risk of data modification
- 🚧 **MCP Server**: Coming soon - native integration with Claude Code and other AI tools
- 🔜 **Write Operations**: Planned for future release with safety controls and audit logging

## Installation

### From Source

```bash
git clone https://github.com/thalesgelinger/dbbridge.git
cd dbbridge
make build
./bin/dbbridge --version
```

### Using Make

```bash
make build          # Build binary
make test           # Run tests
make test-coverage  # Generate coverage report
make db-setup       # Setup test database
make clean          # Remove build artifacts
make help           # Show all commands
```

### Homebrew (Coming Soon)

```bash
brew tap thalesgelinger/tap
brew install dbbridge
```

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

## Output Formats

### Ultra-Compact JSON (for AI Agents)

The default format is optimized for token efficiency:

```bash
# Single value → Just the value
dbbridge query "SELECT count(*) FROM users"
1247  # 5 tokens (vs 450 with verbose JSON)

# Single column → Array
dbbridge query "SELECT email FROM users LIMIT 3"
["alice@example.com","bob@example.com","charlie@example.com"]  # 30 tokens

# Multi-column → Compact array format
dbbridge query "SELECT id, name FROM users LIMIT 2"
{"cols":["id","name"],"rows":[[1,"Alice"],[2,"Bob"]]}  # 180 tokens
```

### Format Options

```bash
# Explicit compact format
dbbridge query "..." --format=compact

# Human-readable table (auto-detected in terminal)
dbbridge query "..."

# Force specific format
dbbridge query "..." --format=table
dbbridge query "..." --format=csv
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

## Project Status

**Phase 1: Core CLI** ✅ **COMPLETED**
- [x] Basic project structure
- [x] Credential storage (OS keychain)
- [x] Configuration management
- [x] Profile management
- [x] Query execution
- [x] Ultra-compact output format
- [x] Table and CSV output formats
- [x] Schema inspection
- [x] Makefile for development

**Phase 2: Multi-Database Support** 🚧 **IN PROGRESS**
- [x] PostgreSQL support
- [ ] MySQL/MariaDB support
- [ ] SQLite support
- [ ] SQL Server support

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
make build
# or
go build -o bin/dbbridge ./cmd/dbbridge
```

### Run Tests

```bash
make test
# or
go test ./...
```

### Test Coverage

```bash
make test-coverage
# Opens coverage/coverage.html in browser
```

### Setup Test Database

```bash
make db-setup
```

### Dependencies

- Go 1.25+
- PostgreSQL 12+ (for PostgreSQL connections)

## Architecture

```
dbbridge/
├── cmd/
│   └── dbbridge/           # Main CLI binary
├── internal/
│   ├── cli/                # CLI commands
│   ├── config/             # Configuration management
│   ├── credentials/        # OS keychain integration
│   ├── database/           # Database client
│   └── output/             # Output formatters
├── test/                   # Tests
└── Makefile               # Build automation
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
