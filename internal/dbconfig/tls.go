package dbconfig

import "strings"

// MySQLTLSMode maps dbridge SSL modes to go-sql-driver/mysql TLS values.
func MySQLTLSMode(sslMode string) string {
	switch strings.ToLower(sslMode) {
	case "disable":
		return "false"
	case "prefer", "preferred":
		return "preferred"
	case "require", "verify-ca", "verify-full", "":
		return "true"
	default:
		return "true"
	}
}

// MSSQLTLSMode maps dbridge SSL modes to go-mssqldb encryption values.
func MSSQLTLSMode(sslMode string) (encrypt string, trust string) {
	switch strings.ToLower(sslMode) {
	case "disable":
		return "disable", ""
	case "prefer", "preferred":
		return "false", ""
	case "require":
		return "true", "true"
	case "verify-ca", "verify-full":
		return "true", "false"
	default:
		return "true", "false"
	}
}
