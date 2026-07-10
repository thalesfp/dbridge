package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/thalesfp/dbridge/internal/dbconfig"
)

func init() { RegisterDriver("mssql", &MssqlDriver{}) }

// MssqlDriver implements Driver for Microsoft SQL Server.
type MssqlDriver struct{}

func (d *MssqlDriver) Connect(ctx context.Context, config *ConnectionConfig) (Connection, error) {
	dsn := buildMssqlDSN(config)

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mssql connection: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &MssqlConnection{
		db:     db,
		conn:   conn,
		config: config,
	}, nil
}

// MssqlConnection implements Connection using database/sql with go-mssqldb.
// Uses a pinned *sql.Conn so SET SHOWPLAN_XML ON/OFF stays on a single connection.
//
// SQL Server has no session-level read-only switch (unlike PostgreSQL's
// default_transaction_read_only or MySQL's SET SESSION TRANSACTION READ ONLY).
// Read-only is enforced two ways: ApplicationIntent=ReadOnly in the DSN (routes
// to a readable secondary in an Always On availability group) and an app-level
// write-statement guard (mssqlCheckReadOnly). The guard is best-effort against
// accidental writes, not a hard security boundary; a hard guarantee requires a
// read-only login (db_datareader).
type MssqlConnection struct {
	db     *sql.DB
	conn   *sql.Conn
	config *ConnectionConfig
	dead   bool // pinned conn was closed after a failed SHOWPLAN cleanup
}

// buildMssqlDSN builds a sqlserver:// URL DSN using net/url for proper escaping.
func buildMssqlDSN(config *ConnectionConfig) string {
	query := url.Values{}
	if config.Database != "" {
		query.Set("database", config.Database)
	}
	query.Set("applicationintent", "ReadOnly")

	encrypt, trust := mapSSLToMssql(config.SSLMode)
	query.Set("encrypt", encrypt)
	if trust != "" {
		query.Set("trustservercertificate", trust)
	}

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(config.Username, config.Password),
		Host:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		RawQuery: query.Encode(),
	}
	return u.String()
}

// mapSSLToMssql maps dbridge ssl_mode values to go-mssqldb encrypt and
// trustservercertificate parameters.
//
// ConnectionConfig has no CA/cert-path field, so true CA-only validation is not
// possible; verify-ca is treated as an alias of verify-full (encrypt + full
// validation).
func mapSSLToMssql(sslMode string) (encrypt string, trust string) {
	return dbconfig.MSSQLTLSMode(sslMode)
}

// mssqlReadLeaders are the only statement-leading keywords a read query may start with.
var mssqlReadLeaders = map[string]bool{
	"select": true,
	"with":   true,
	"values": true,
}

// mssqlWriteTokens are SQL Server RESERVED keywords that begin or constitute a
// write, DDL, or server/session state change, so they must never appear anywhere in
// a read query. The set is drawn from the reserved-keyword list and groups into:
// DML/DDL, permissions (grant/revoke/deny), backup/restore (incl. legacy
// dump/load), admin & session (use/set/waitfor/kill/shutdown/reconfigure/dbcc/
// checkpoint), transaction control (begin/commit/rollback/save), impersonation
// (setuser/revert), cursor/key management (open/close, which can change
// session-bound cryptographic key state), procedure execution (exec/execute), the
// distributed-query
// functions (openquery/openrowset/opendatasource) which can hide writes in opaque
// string arguments, the legacy text writes (writetext/updatetext) and trigger
// management (the "trigger" keyword, which only appears in *.TRIGGER statements).
//
// Because every entry is reserved, it can never be a bare identifier in a real read
// (those must be bracketed or quoted, which the tokenizer skips). These catch
// dangerous constructs stacked WITHOUT a separator (T-SQL allows "SELECT 1 WAITFOR
// DELAY ..."); semicolon-separated batches are rejected separately by
// mssqlCheckReadOnly. NB: the reserved keyword "fetch" is intentionally excluded
// because it is part of the read-only OFFSET/FETCH pagination clause.
//
// Lock hints (updlock/xlock/tablockx/holdlock/serializable/tablock/...) are
// deliberately NOT policed: they acquire locks but do not mutate data, every query
// runs in autocommit so the locks release when the statement finishes, and most are
// not reserved, so denylisting them would reject valid identifiers such as
// "SELECT updlock FROM t". Telling a table-hint clause from an identifier needs a
// real parser, out of proportion for a bounded, non-mutating effect.
var mssqlWriteTokens = map[string]bool{
	"insert": true, "update": true, "delete": true, "merge": true,
	"drop": true, "create": true, "alter": true, "truncate": true,
	"into": true, "exec": true, "execute": true, "grant": true,
	"revoke": true, "deny": true, "backup": true, "restore": true,
	"dump": true, "load": true, "dbcc": true, "bulk": true,
	"declare": true, "use": true, "set": true, "waitfor": true,
	"kill": true, "shutdown": true, "reconfigure": true, "checkpoint": true,
	"begin": true, "commit": true, "rollback": true, "save": true,
	"setuser": true, "revert": true, "open": true, "close": true,
	"openquery": true, "openrowset": true, "opendatasource": true,
	"writetext": true, "updatetext": true, "trigger": true,
}

var (
	mssqlReadOnlyErr = errors.New("read-only connection: only read queries are permitted")
	mssqlDeadConnErr = errors.New("connection is closed after a failed showplan cleanup")
)

// mssqlCheckReadOnly rejects any statement that is not provably a read.
//
// It tokenizes the query (skipping string literals, quoted/bracketed identifiers
// and comments, and emitting ";" as a statement separator), then applies these
// rules:
//
//  1. The first word token must be select/with/values.
//  2. No second statement: once a ";" separator follows the first statement, any
//     further word token rejects the batch. This blocks session/admin commands
//     stacked after a read (e.g. "SELECT 1; USE db", "SELECT 1; KILL 52",
//     "SELECT 1; WAITFOR DELAY ...") even when their verb is not in the denylist.
//     A leading ";" (the common ";WITH cte" idiom) and a trailing ";" are fine.
//  3. No write/exec keyword (mssqlWriteTokens) or sp_/xp_ procedure name appears
//     anywhere. This catches statements stacked WITHOUT a separator, which T-SQL
//     allows ("SELECT 1 DROP TABLE t"), plus SELECT...INTO, CTE writes and the
//     DECLARE/EXEC sp_executesql bypass. Real identifiers with those names must
//     be bracketed or quoted, which the tokenizer skips.
//  4. The sequence-advancing phrase "NEXT VALUE FOR" is rejected. It is a valid
//     read expression but mutates server state (advances a sequence). The trailing
//     "for" is required so columns literally named "next"/"value" stay allowed;
//     bracketing them sidesteps the check entirely.
//
// Semicolon-less all-read batches ("SELECT 1 SELECT 2") are not detectable without
// a parser and are harmless, so they are allowed. This is best-effort protection
// against accidental writes, not a hard security boundary.
func mssqlCheckReadOnly(query string) error {
	tokens := mssqlTokenize(query)

	seenWord := false
	sepAfterWord := false
	var prev2, prev1 string

	for _, tok := range tokens {
		if tok == ";" {
			if seenWord {
				sepAfterWord = true
			}
			continue
		}

		low := strings.ToLower(tok)

		if !seenWord {
			seenWord = true
			if !mssqlReadLeaders[low] {
				return mssqlReadOnlyErr
			}
		} else if sepAfterWord {
			return mssqlReadOnlyErr
		}

		if mssqlWriteTokens[low] {
			return mssqlReadOnlyErr
		}
		if strings.HasPrefix(low, "sp_") || strings.HasPrefix(low, "xp_") {
			return mssqlReadOnlyErr
		}
		if prev2 == "next" && prev1 == "value" && low == "for" {
			return mssqlReadOnlyErr
		}

		prev2, prev1 = prev1, low
	}

	if !seenWord {
		return mssqlReadOnlyErr
	}

	return nil
}

// mssqlTokenize splits a T-SQL string into word tokens ([A-Za-z0-9_@#]+) plus ";"
// separator tokens, skipping string literals ('...'), quoted identifiers ("..."),
// bracketed identifiers ([...]) and comments (-- and /* */, including nested block
// comments). Keywords hidden inside any of those regions are not returned, so they
// cannot trip the guard; ";" is emitted so the caller can detect statement boundaries.
func mssqlTokenize(query string) []string {
	var tokens []string
	var cur strings.Builder
	runes := []rune(query)
	n := len(runes)

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	isWord := func(r rune) bool {
		return r == '_' || r == '@' || r == '#' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')
	}

	for i := 0; i < n; {
		r := runes[i]

		switch {
		case r == '-' && i+1 < n && runes[i+1] == '-':
			flush()
			i += 2
			// T-SQL ends a "--" comment at a line feed OR a bare carriage
			// return (or CRLF). Stopping only at '\n' would let a CR-only line
			// ending hide a following statement, e.g. "SELECT 1 -- x\rDROP ...".
			for i < n && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}

		case r == '/' && i+1 < n && runes[i+1] == '*':
			flush()
			i += 2
			depth := 1
			for i < n && depth > 0 {
				if runes[i] == '/' && i+1 < n && runes[i+1] == '*' {
					depth++
					i += 2
				} else if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					depth--
					i += 2
				} else {
					i++
				}
			}

		case r == '\'':
			flush()
			i = mssqlSkipQuoted(runes, i, '\'')

		case r == '"':
			flush()
			i = mssqlSkipQuoted(runes, i, '"')

		case r == '[':
			flush()
			i = mssqlSkipQuoted(runes, i, ']')

		case r == ';':
			flush()
			tokens = append(tokens, ";")
			i++

		case isWord(r):
			cur.WriteRune(r)
			i++

		default:
			flush()
			i++
		}
	}

	flush()
	return tokens
}

// mssqlSkipQuoted skips a delimited region that starts at runes[start]. The opening
// delimiter is runes[start]; close is the given rune. A doubled close delimiter is an
// escape (a doubled quote inside a string, or a doubled bracket inside an identifier).
// Returns the index just past the closing delimiter.
func mssqlSkipQuoted(runes []rune, start int, closeDelim rune) int {
	n := len(runes)
	i := start + 1
	for i < n {
		if runes[i] == closeDelim {
			if i+1 < n && runes[i+1] == closeDelim {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return n
}

// mssqlTypeName normalizes a SQL Server type name to a short lowercase name.
func mssqlTypeName(typ string) string {
	t := strings.ToLower(strings.TrimSpace(typ))
	if idx := strings.Index(t, "("); idx != -1 {
		t = strings.TrimSpace(t[:idx])
	}

	switch t {
	case "int":
		return "int4"
	case "bigint":
		return "int8"
	case "smallint":
		return "int2"
	case "tinyint":
		return "int1"
	case "bit":
		return "bool"
	case "decimal", "numeric", "money", "smallmoney":
		return "numeric"
	case "float":
		return "float8"
	case "real":
		return "float4"
	case "uniqueidentifier":
		return "uuid"
	case "rowversion", "timestamp":
		// SQL Server "timestamp" is a binary rowversion alias, not a temporal type.
		return "rowversion"
	case "char", "nchar", "varchar", "nvarchar", "text", "ntext",
		"date", "time", "datetime", "datetime2", "smalldatetime", "datetimeoffset",
		"binary", "varbinary", "image",
		"xml", "hierarchyid", "geography", "geometry", "sql_variant":
		return t
	default:
		return t
	}
}

// Query executes a read-only SELECT query.
func (c *MssqlConnection) Query(ctx context.Context, sqlStr string, args ...interface{}) (*QueryResult, error) {
	if c.dead {
		return nil, mssqlDeadConnErr
	}
	if err := mssqlCheckReadOnly(sqlStr); err != nil {
		return nil, err
	}

	start := time.Now()

	rows, err := c.conn.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	columns := make([]string, len(colTypes))
	columnTypeNames := make([]string, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ct.Name()
		columnTypeNames[i] = mssqlTypeName(ct.DatabaseTypeName())
	}

	dest := make([]interface{}, len(colTypes))
	for i := range dest {
		dest[i] = new(interface{})
	}

	var result [][]interface{}
	truncated := false
	for rows.Next() {
		if len(result) >= maxSQLRows {
			truncated = true
			break
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make([]interface{}, len(colTypes))
		for i, d := range dest {
			val := *(d.(*interface{}))
			if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = val
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	duration := time.Since(start)

	qr := &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypeNames,
		Rows:        result,
		RowCount:    len(result),
		Duration:    duration,
		Truncated:   truncated,
	}
	if truncated {
		qr.Warnings = append(qr.Warnings, fmt.Sprintf(truncatedWarning, maxSQLRows))
	}
	return qr, nil
}

// Exec is rejected: dbridge connections are read-only and SQL Server cannot enforce
// that at the session level, so writes are refused in application code.
func (c *MssqlConnection) Exec(ctx context.Context, sqlStr string, args ...interface{}) (*ExecResult, error) {
	return nil, mssqlReadOnlyErr
}

// Schema returns schema inspector.
func (c *MssqlConnection) Schema() SchemaInspector {
	return &MssqlSchemaInspector{conn: c}
}

// Close closes the pinned connection and the underlying pool.
func (c *MssqlConnection) Close(ctx context.Context) error {
	c.conn.Close()
	return c.db.Close()
}

// Config returns connection configuration (without password).
func (c *MssqlConnection) Config() *ConnectionConfig {
	config := *c.config
	config.Password = "***"
	return &config
}

// MssqlSchemaInspector implements SchemaInspector for SQL Server.
// SQL Server has real schemas (dbo, etc.) inside the connected database, so this
// follows the PostgreSQL model rather than collapsing schema into the database name.
type MssqlSchemaInspector struct {
	conn *MssqlConnection
}

// mssqlSchemaOrDefault maps the postgres-centric default ("public" or empty) to the
// SQL Server default schema (dbo).
func mssqlSchemaOrDefault(schema string) string {
	if schema == "" || schema == "public" {
		return "dbo"
	}
	return schema
}

// ListSchemas lists user schemas, excluding the built-in system schemas.
func (s *MssqlSchemaInspector) ListSchemas(ctx context.Context) ([]Schema, error) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN (
			'sys', 'INFORMATION_SCHEMA', 'guest',
			'db_owner', 'db_accessadmin', 'db_securityadmin', 'db_ddladmin',
			'db_backupoperator', 'db_datareader', 'db_datawriter',
			'db_denydatareader', 'db_denydatawriter'
		)
		ORDER BY schema_name
	`

	rows, err := s.conn.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []Schema
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		schemas = append(schemas, Schema{Name: name})
	}

	return schemas, rows.Err()
}

// ListTables lists tables in a schema (defaults to dbo).
func (s *MssqlSchemaInspector) ListTables(ctx context.Context, schema string) ([]Table, error) {
	schema = mssqlSchemaOrDefault(schema)

	query := `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema = @p1 AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := s.conn.conn.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Schema, &t.Name); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// DescribeTable gets table structure using information_schema.columns.
func (s *MssqlSchemaInspector) DescribeTable(ctx context.Context, schema, table string) (*TableDefinition, error) {
	schema = mssqlSchemaOrDefault(schema)

	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = @p1 AND table_name = @p2
		ORDER BY ordinal_position
	`

	rows, err := s.conn.conn.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &col.Default); err != nil {
			return nil, err
		}
		col.Nullable = nullable == "YES"
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TableDefinition{
		Schema:  schema,
		Name:    table,
		Columns: columns,
	}, nil
}

// ExplainQuery returns the estimated execution plan as XML.
//
// T-SQL has no EXPLAIN. SET SHOWPLAN_XML ON makes the server return the estimated
// plan for the next statement as XML instead of executing it. It is a session setting
// that must be turned back OFF on the same (pinned) connection; if the OFF fails the
// connection is poisoned (every later query would return plans instead of results),
// so on cleanup failure the pinned connection is closed and marked dead.
func (s *MssqlSchemaInspector) ExplainQuery(ctx context.Context, sqlStr string) (*ExplainResult, error) {
	if s.conn.dead {
		return nil, mssqlDeadConnErr
	}
	if err := mssqlCheckReadOnly(sqlStr); err != nil {
		return nil, err
	}

	conn := s.conn.conn

	if _, err := conn.ExecContext(ctx, "SET SHOWPLAN_XML ON"); err != nil {
		return nil, fmt.Errorf("failed to enable showplan: %w", err)
	}

	plan, planErr := s.fetchPlan(ctx, sqlStr)

	if _, offErr := conn.ExecContext(ctx, "SET SHOWPLAN_XML OFF"); offErr != nil {
		s.conn.dead = true
		conn.Close()
		if planErr != nil {
			return nil, fmt.Errorf("showplan failed (%v) and could not be disabled, connection closed: %w", planErr, offErr)
		}
		return nil, fmt.Errorf("failed to disable showplan, connection closed: %w", offErr)
	}

	if planErr != nil {
		return nil, planErr
	}

	return &ExplainResult{Plan: plan}, nil
}

// fetchPlan runs the query while SHOWPLAN_XML is on and reads the single XML plan row.
func (s *MssqlSchemaInspector) fetchPlan(ctx context.Context, sqlStr string) (string, error) {
	rows, err := s.conn.conn.QueryContext(ctx, sqlStr)
	if err != nil {
		return "", fmt.Errorf("failed to get query plan: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("SHOWPLAN_XML returned no rows")
	}

	var plan string
	if err := rows.Scan(&plan); err != nil {
		return "", fmt.Errorf("failed to scan plan: %w", err)
	}

	return plan, nil
}
