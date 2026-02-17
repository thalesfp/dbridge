# dbridge - Database Bridge for AI Agents

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
git clone https://github.com/thalesfp/dbridge.git
cd dbridge
make build
./bin/dbridge --version
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
dbridge config add local \
  --host=localhost \
  --database=mydb \
  --username=admin
# Enter password when prompted
```

### 2. Execute Queries

```bash
# Simple count
dbridge query local "SELECT count(*) FROM users"
# Output: 1247

# List emails
dbridge query local "SELECT email FROM users LIMIT 3"
# Output: ["alice@example.com", "bob@example.com", "charlie@example.com"]

# Multi-column result
dbridge query local "SELECT id, name, active FROM users LIMIT 2"
# Output: {"cols":["id","name","active"],"rows":[[1,"Alice",true],[2,"Bob",false]]}
```

### 3. Manage Profiles

```bash
# List all profiles
dbridge config list

# Show profile details
dbridge config show local

# Remove profile
dbridge config remove staging
```

## MCP Server

dbridge includes an MCP server that exposes read-only database tools over stdio.

**Claude Desktop** — add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "dbridge": {
      "command": "/path/to/dbridge",
      "args": ["mcp"]
    }
  }
}
```

**Claude Code:**

```bash
claude mcp add dbridge -- /path/to/dbridge mcp
```

**Codex** — add to `~/.codex/config.toml`:

```toml
[mcp_servers.dbridge]
command = "/path/to/dbridge"
args = ["mcp"]
```

Or via CLI:

```bash
codex mcp add dbridge -- /path/to/dbridge mcp
```

**Available Tools:**

| Tool | Description |
|------|-------------|
| `query` | Execute a read-only SQL query |
| `list_profiles` | List configured database profiles |
| `list_schemas` | List schemas in a database |
| `list_tables` | List tables in a schema |
| `describe_table` | Describe table structure |
| `explain_query` | Show query execution plan |

## Configuration

Configuration file: `~/.config/dbridge/config.yaml`

```yaml
settings:
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
