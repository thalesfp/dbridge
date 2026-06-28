# dbridge - Database Bridge for AI Agents

A cross-platform database CLI tool with MCP (Model Context Protocol) server support, designed to bridge AI agents like Claude Code to databases.

## Features

- **Multi-Database Support**: PostgreSQL, MySQL, MongoDB, SQL Server
- **Secure Credential Storage**: Uses OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service). Passwords are never stored in config files or logs.
- **Multi-Connection Management**: Manage multiple database connections with named connections
- **Multiple Output Formats**: Compact JSON (token-efficient for AI agents), table, and CSV
- **Schema Inspection**: List schemas, tables, and describe table structures
- **Always Read-Only**: All connections are read-only by design. PostgreSQL uses `default_transaction_read_only=on` in the connection string, MySQL issues `SET SESSION TRANSACTION READ ONLY` on every connection, MongoDB blocks write pipeline stages (`$out`, `$merge`) and server-side JavaScript operators (`$where`, `$function`, `$accumulator`) in application code, and SQL Server sets `ApplicationIntent=ReadOnly` plus an application-level write-statement guard. There is no config option to disable this. (SQL Server has no session-level read-only switch, so its guarantee is weaker than the others — see [Read-Only Enforcement](#read-only-enforcement).)

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

- Go 1.25+
- PostgreSQL 12+, MySQL 8.0+, MongoDB 4.4+, and/or SQL Server 2017+ (depending on which databases you connect to)

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

### Interactive Connection Manager

Run `dbridge config` without subcommands to open the interactive TUI:

```bash
dbridge config
```

The TUI lets you add, edit, enable/disable, and delete connections with keyboard navigation (`↑↓` move, `enter` edit, `a` add, `d` delete, `t` toggle, `esc` quit).

## MCP Server

dbridge includes an MCP server that exposes read-only database tools over stdio.

**Claude Desktop** - add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

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

**Codex** - add to `~/.codex/config.toml`:

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
    environment: "dev"        # optional: dev, staging, production
    description: "Local dev"  # optional: free-text label
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

  # SQL Server
  mssql-local:
    driver: "mssql"
    host: "localhost"
    port: 1433
    database: "myapp"
    username: "sa"
    ssl_mode: "verify-full"
```

## Read-Only Enforcement

Every dbridge connection is read-only by design:

- **PostgreSQL** - `default_transaction_read_only=on` is appended to every connection string. The server rejects any write statement before it executes.
- **MySQL** - `SET SESSION TRANSACTION READ ONLY` is issued on the pinned connection immediately after opening. Writes fail at the session level.
- **MongoDB** - Write pipeline stages (`$out`, `$merge`) and server-side JavaScript operators (`$where`, `$function`, `$accumulator`) are blocked before the query is sent to the server.
- **SQL Server** - `ApplicationIntent=ReadOnly` is set on the connection (which routes to a readable secondary in an Always On availability group), and every query is screened by an application-level guard that rejects anything that is not a plain read (`INSERT`, `UPDATE`, `DELETE`, `MERGE`, `SELECT ... INTO`, `EXEC`/`sp_executesql`, DDL, stacked statements, etc.). `Exec` is refused outright.

> **SQL Server caveat:** unlike PostgreSQL and MySQL, SQL Server has no session-level read-only switch. The guard above is best-effort protection against accidental writes, **not** a hardened security boundary. For a guaranteed read-only connection, point dbridge at a login that only has read permissions (e.g. a user in the `db_datareader` role).

There is no `readonly` config field, no `--readonly` flag, and no way to open a writable connection through dbridge.

## Credential Storage

Credentials are stored in a **dedicated OS keychain named `dbridge`**, separate from your login keychain. The underlying library is [`99designs/keyring`](https://github.com/99designs/keyring), which maps to:

- **macOS** - Keychain Access (`dbridge.keychain-db`)
- **Windows** - Windows Credential Manager
- **Linux** - Secret Service (GNOME Keyring / KWallet)

Each connection's credentials are stored as `dbridge-<connection-name>` (e.g. `dbridge-local`). The config file (`~/.config/dbridge/config.yaml`) never contains passwords.

### Locked keychain during queries

If the keychain is locked when you run a query or schema command, dbridge returns an error rather than attempting a passwordless connection. Unlock the keychain first, or add the connection without a password if passwordless auth is intended.

### Why am I prompted for a password again?

The dedicated `dbridge` keychain auto-locks after a period of inactivity or when the system sleeps. When it locks, dbridge will prompt for the keychain password to unlock it.

To adjust the lock timeout on macOS:

- Open **Keychain Access** -> select the `dbridge` keychain -> right-click **Change Settings for Keychain** -> set the idle timeout or disable "Lock when sleeping"
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
