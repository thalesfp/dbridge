# dbridge

**A read-only database bridge for AI agents.** One CLI and MCP server that connects Claude Code, Claude Desktop, Codex, and other agents to PostgreSQL, MySQL, MongoDB, and SQL Server.

dbridge is built around three ideas:

1. **Agents should never be able to write to your database.** Every connection is read-only by design. There is no flag, config option, or query trick to open a writable connection.
2. **Credentials don't belong in config files.** Passwords live in the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service), never in YAML, environment variables, or process arguments.
3. **Output should be cheap to read for a model.** Results are returned as compact JSON that collapses to a bare value, array, or object when the shape allows, saving tokens on every query.

## Supported databases

| Database   | Driver name | Default port | Read-only mechanism |
|------------|-------------|--------------|---------------------|
| PostgreSQL | `postgres`  | 5432         | `default_transaction_read_only=on`, enforced by the server |
| MySQL      | `mysql`     | 3306         | `SET SESSION TRANSACTION READ ONLY`, enforced by the server |
| MongoDB    | `mongodb`   | 27017        | Write stages and server-side JavaScript rejected client-side |
| SQL Server | `mssql`     | 1433         | `ApplicationIntent=ReadOnly` plus a client-side statement guard |

See [Read-only enforcement](#read-only-enforcement) for the details and one SQL Server caveat.

## Installation

### Homebrew

```bash
brew tap thalesfp/dbridge
brew install dbridge
```

### From source

Requires Go 1.25+.

```bash
git clone https://github.com/thalesfp/dbridge.git
cd dbridge
make build            # binary at ./bin/dbridge
make install          # optional: copy to /usr/local/bin
```

## Quick start

### 1. Add a connection

Run `dbridge config add` in a terminal to get an interactive form with validation, driver-aware defaults, and a built-in connection test (`ctrl+t`).

For scripts and non-interactive use, pass flags instead. `--driver`, `--database`, and `--username` are required; the password is prompted securely (press Enter for passwordless auth):

```bash
dbridge config add local \
  --driver=postgres \
  --host=localhost \
  --database=mydb \
  --username=admin \
  --test-connection
```

Useful optional flags: `--port` and `--ssl-mode` (default to driver-specific values), `--environment` (production, staging, development, local), and `--description`.

### 2. Query

```bash
# Single value: returned as a bare scalar
dbridge query local "SELECT count(*) FROM users"
# 1247

# Single column: returned as an array
dbridge query local "SELECT email FROM users LIMIT 3"
# ["alice@example.com", "bob@example.com", "charlie@example.com"]

# Multiple rows and columns: compact cols/rows format
dbridge query local "SELECT id, name, active FROM users LIMIT 2"
# {"cols":["id","name","active"],"types":["int4","varchar","bool"],"rows":[[1,"Alice",true],[2,"Bob",false]]}
```

### 3. Inspect schema

```bash
dbridge schema list-schemas local
dbridge schema list-tables local --schema public
dbridge schema describe local users
```

### 4. Manage connections

```bash
dbridge config              # interactive TUI: add, edit, clone, enable/disable, delete
dbridge config list         # list all connections
dbridge config clone local staging
dbridge config remove staging
```

Removing a connection also deletes its credentials from the keychain.

## MCP server

`dbridge mcp` starts a Model Context Protocol server over stdio, exposing the same read-only tools to any MCP client.

**Claude Code:**

```bash
claude mcp add dbridge -- /path/to/dbridge mcp
```

**Claude Desktop**, in `~/Library/Application Support/Claude/claude_desktop_config.json`:

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

**Codex:**

```bash
codex mcp add dbridge -- /path/to/dbridge mcp
```

### Tools

| Tool | Description |
|------|-------------|
| `query` | Execute a read-only query (SQL, or a JSON query object for MongoDB) |
| `list_connections` | List configured database connections |
| `list_schemas` | List schemas in a database |
| `list_tables` | List tables in a schema |
| `describe_table` | Describe a table's columns, indexes, and constraints |
| `explain_query` | Show a query's execution plan |

All tools are annotated as read-only and non-destructive, and the server instructs agents to prefer dbridge over shelling out to `psql`, `mysql`, `mongosh`, or `sqlcmd`.

## Querying MongoDB

For MongoDB connections, `query` takes a JSON object instead of SQL:

```bash
# Find with filter, projection, sort, skip, limit
dbridge query mongo '{"collection": "users", "filter": {"active": true}, "projection": {"name": 1, "email": 1}, "sort": {"name": 1}, "limit": 5}'

# Aggregation pipeline
dbridge query mongo '{"collection": "orders", "aggregate": [{"$match": {"status": "completed"}}, {"$group": {"_id": "$product", "total": {"$sum": "$amount"}}}]}'
```

Find queries default to a 1,000-document limit when none is given.

## Output

dbridge adapts its output to who is reading it:

- **In a terminal** (stdout and stdin are TTYs): human-readable output, and `dbridge config` opens the interactive TUI.
- **Piped or driven by an agent**: structured JSON everywhere, including help text and errors.
- Override either way with the global `--json` or `--human` flags.

### Query formats

`dbridge query` accepts `--format` (`-f`): `compact` (default), `table`, `table-compact`, or `csv`.

The compact format is `{"cols":[...],"types":[...],"rows":[[...]]}` with optional fields: `t` (execution time in ms) and `w` (warnings). Truncation is signaled by a `w` warning; there is also a reserved `n` field for the true total row count, but drivers cannot know it once results are capped, so it is not currently emitted. Smart simplification collapses simple shapes: one row and one column becomes a bare value, one column becomes an array, one row becomes an object. When a result is truncated, simplification is skipped so the warning is not lost.

SQL results are capped at 10,000 rows; a truncation warning tells the agent to add a `LIMIT`. Query errors are written to stderr as compact JSON (with hint, error code, and position when the server provides them) so they never mix into the data stream.

### Output settings

In `~/.config/dbridge/config.yaml`:

```yaml
settings:
  output:
    default: "auto"          # auto, compact, table, csv
    auto_detect_tty: true    # switch between human/JSON based on TTY
    include_types: true      # include column types in compact output
    include_timing: false    # include execution time (costs tokens)
    include_warnings: true   # include warnings such as truncation notices
    smart_simplify: true     # collapse single value/column/row results
```

## Configuration

Connections are stored in `~/.config/dbridge/config.yaml` (written with `0600` permissions, and never containing passwords):

```yaml
connections:
  local:
    driver: "postgres"
    host: "localhost"
    port: 5432
    database: "mydb"
    username: "admin"
    ssl_mode: "prefer"
    environment: "development"   # optional: production, staging, development, local
    description: "Local dev"     # optional free-text label

  mysql-local:
    driver: "mysql"
    host: "localhost"
    port: 3306
    database: "myapp"
    username: "app"

  # MongoDB (standard)
  mongo-local:
    driver: "mongodb"
    host: "localhost"
    port: 27017
    database: "myapp"
    username: "admin"
    ssl_mode: "disable"

  # MongoDB Atlas (SRV)
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

Field notes:

- `driver` defaults to `postgres` when omitted (backward compatibility with older configs).
- `ssl_mode` uses PostgreSQL-style values (`disable`, `prefer`, `require`, `verify-ca`, `verify-full`) and is mapped to each driver's native TLS settings. The default is `verify-full` for every driver. `require` encrypts the connection but does not verify the server certificate; `verify-ca` and `verify-full` additionally verify it.
  - **Upgrade note (MySQL):** through v0.6.0, MySQL `require` verified the server certificate. It now matches the PostgreSQL/libpq meaning (encrypt only, no verification), so `require` and `verify-full` are no longer equivalent for MySQL. If you relied on `require` to authenticate a MySQL server, switch that connection to `verify-full`. dbridge prints a warning when a MySQL connection uses a mode that does not verify the server certificate (`require` or `prefer`).
- `uri` (MongoDB) lets you supply a full connection URI instead of host/port fields.
- `srv` (MongoDB) switches to `mongodb+srv://` for Atlas-style DNS seed lists.
- `disabled: true` keeps a connection configured but refuses queries against it. Toggle it from the `dbridge config` TUI.

## Read-only enforcement

Every dbridge connection is read-only. There is no `readonly` config field, no `--readonly` flag, and no way to open a writable connection.

- **PostgreSQL**: `default_transaction_read_only=on` is appended to every connection string. The server itself rejects any write statement.
- **MySQL**: `SET SESSION TRANSACTION READ ONLY` is issued on the pinned connection immediately after opening. Writes fail at the session level.
- **MongoDB**: write pipeline stages (`$out`, `$merge`) and server-side JavaScript operators (`$where`, `$function`, `$accumulator`) are rejected before the query is sent to the server.
- **SQL Server**: `ApplicationIntent=ReadOnly` is set on the connection (which also routes to a readable secondary in an Always On availability group), and every statement is screened by a client-side guard that rejects anything that is not a plain read: `INSERT`, `UPDATE`, `DELETE`, `MERGE`, `SELECT ... INTO`, `EXEC`/`sp_executesql`, DDL, stacked statements, and so on.

> **SQL Server caveat:** unlike PostgreSQL and MySQL, SQL Server has no session-level read-only switch, so its guard is best-effort protection against accidental writes rather than a hardened security boundary. For a guaranteed read-only connection, point dbridge at a login that only has read permissions (for example, a user in the `db_datareader` role).

For defense in depth on any database, use a dedicated read-only database user. dbridge's enforcement is a safety layer on top of, not a replacement for, database-level permissions.

## Credential storage

Passwords are stored in a **dedicated OS keychain named `dbridge`**, separate from your login keychain, via [`99designs/keyring`](https://github.com/99designs/keyring):

- **macOS**: Keychain Access (`dbridge.keychain-db`)
- **Windows**: Credential Manager
- **Linux**: Secret Service (GNOME Keyring / KWallet)

Each connection's password is stored under `dbridge-<connection-name>`. The config file never contains passwords, and the username in the config file is the source of truth (the keychain only supplies the secret).

If the keychain is locked when a query runs, dbridge returns an error instead of silently attempting a passwordless connection.

### Why am I prompted for a password again?

The dedicated `dbridge` keychain auto-locks after a period of inactivity or when the system sleeps. To adjust the timeout on macOS:

```bash
# Lock after 1 hour of inactivity, don't lock on sleep
security set-keychain-settings -t 3600 ~/Library/Keychains/dbridge.keychain-db

# Never auto-lock (not recommended)
security set-keychain-settings ~/Library/Keychains/dbridge.keychain-db
```

Or in Keychain Access: right-click the `dbridge` keychain and choose "Change Settings for Keychain".

## Development

```bash
make build            # build binary to bin/dbridge
make test             # run unit tests
make docker-up        # start test containers (Postgres x4 incl. SSL, MySQL 8, MongoDB 7, SQL Server 2022)
make test-integration # run integration tests against the containers
make test-coverage    # unit tests with race detector and HTML coverage report
make lint             # golangci-lint
make fmt              # go fmt
make docker-down      # stop containers and remove volumes
make clean            # remove build artifacts
```

The Docker test environment (see `docker-compose.yml`) includes PostgreSQL variants for password, trust, and SSL authentication plus a rich-schema instance, all seeded with fixtures from `test/fixtures/`.

## License

MIT. See [LICENSE](LICENSE).
