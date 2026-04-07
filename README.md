# dbridge - Database Bridge for AI Agents

A cross-platform database CLI tool with MCP (Model Context Protocol) server support, designed to bridge AI agents like Claude Code to databases.

## Features

- **Multi-Database Support**: PostgreSQL, MySQL, MongoDB
- **Secure Credential Storage**: Uses OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service) — passwords never stored in config files or logs
- **Multi-Connection Management**: Manage multiple database connections with named connections
- **Multiple Output Formats**: Compact JSON (token-efficient for AI agents), table, and CSV
- **Schema Inspection**: List schemas, tables, and describe table structures
- **Read-Only Mode**: Safe query execution without risk of data modification

## Installation

### Homebrew

```bash
brew tap thalesfp/dbridge
brew install dbridge
```

### From Source

```bash
git clone https://github.com/thalesfp/dbridge.git
cd dbridge
make build
./bin/dbridge --version
```

### Make Commands

```bash
make build            # Build binary
make test             # Run unit tests
make test-integration # Run integration tests (requires make docker-up)
make test-coverage    # Generate coverage report
make lint             # Run golangci-lint
make fmt              # Format code
make docker-up        # Start test database containers
make docker-down      # Stop test database containers
make clean            # Remove build artifacts
```

### Dependencies

- Go 1.24+
- PostgreSQL 12+, MySQL 8.0+, and/or MongoDB 4.4+ (depending on which databases you connect to)

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

### MongoDB Queries

For MongoDB connections, the `query` tool accepts a JSON object instead of SQL:

```bash
# Find documents
dbridge query mongo '{"collection": "users", "filter": {"active": true}, "limit": 5}'

# With projection and sort
dbridge query mongo '{"collection": "users", "filter": {}, "projection": {"name": 1, "email": 1}, "sort": {"name": 1}}'

# Aggregation pipeline
dbridge query mongo '{"collection": "orders", "aggregate": [{"$match": {"status": "completed"}}, {"$group": {"_id": "$product", "total": {"$sum": "$amount"}}}]}'
```

### 3. Manage Connections

```bash
# List all connections
dbridge config list

# Remove connection
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
| `query` | Execute a read-only query (SQL or MongoDB JSON) |
| `list_connections` | List configured database connections |
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

connections:
  local:
    host: "localhost"
    port: 5432
    database: "mydb"
    username: "admin"
    ssl_mode: "prefer"
    readonly: false
    # Password stored in OS Keychain

  # MongoDB (standard)
  mongo-local:
    driver: "mongodb"
    host: "localhost"
    port: 27017
    database: "myapp"
    username: "admin"
    ssl_mode: "disable"

  # MongoDB (SRV / Atlas)
  mongo-atlas:
    driver: "mongodb"
    host: "cluster.example.mongodb.net"
    database: "myapp"
    username: "admin"
    ssl_mode: "require"
    srv: true
```

## Credential Storage

Credentials are stored in a **dedicated OS keychain named `dbridge`**, separate from your login keychain. The underlying library is [`99designs/keyring`](https://github.com/99designs/keyring), which maps to:

- **macOS** — Keychain Access (`dbridge.keychain-db`)
- **Windows** — Windows Credential Manager
- **Linux** — Secret Service (GNOME Keyring / KWallet)

Each connection's credentials are stored as `dbridge-<connection-name>` (e.g. `dbridge-local`). The config file (`~/.config/dbridge/config.yaml`) never contains passwords.

### Why am I prompted for a password again?

The dedicated `dbridge` keychain auto-locks after a period of inactivity or when the system sleeps. When it locks, dbridge will prompt for the keychain password to unlock it.

To adjust the lock timeout on macOS:

- Open **Keychain Access** → select the `dbridge` keychain → right-click **Change Settings for Keychain** → set the idle timeout or disable "Lock when sleeping"
- Or via terminal:
  ```bash
  # Lock after 1 hour (3600 seconds) of inactivity, don't lock on sleep
  security set-keychain-settings -t 3600 ~/Library/Keychains/dbridge.keychain-db

  # Never auto-lock (not recommended)
  security set-keychain-settings ~/Library/Keychains/dbridge.keychain-db
  ```

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please open an issue or PR.
