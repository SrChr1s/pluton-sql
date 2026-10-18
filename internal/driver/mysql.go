package driver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

type mysqlDialect struct{}

func init() { register(mysqlDialect{}) }

func (mysqlDialect) Name() string       { return "mysql" }
func (mysqlDialect) DriverName() string { return "mysql" }
func (mysqlDialect) DefaultPort() int   { return 3306 }

func (mysqlDialect) DSN(p config.ConnectionProfile, password, database string) string {
	port := p.Port
	if port == 0 {
		port = 3306
	}
	cfg := mysql.NewConfig()
	cfg.User = p.User
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", p.Host, port)
	cfg.DBName = pick(database, p.Database)
	cfg.ParseTime = true
	if v := legacyTLS(p.TLSMinVersion); v != "" {
		name := "pluton-tls-" + v
		if err := mysql.RegisterTLSConfig(name, legacyTLSConfig(tlsVersions[v])); err == nil {
			cfg.TLSConfig = name
		}
	}
	return cfg.FormatDSN()
}

func (mysqlDialect) ListDatabases(ctx context.Context, db *sql.DB) ([]string, error) {
	return queryStrings(ctx, db, `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema','performance_schema','mysql','sys') ORDER BY schema_name`)
}

// MySQL no tiene nivel de esquema separado de la base de datos.
func (mysqlDialect) ListSchemas(context.Context, *sql.DB) ([]string, error) { return []string{""}, nil }

func (mysqlDialect) ListTables(ctx context.Context, db *sql.DB, _ string) ([]TableInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT table_name, table_type FROM information_schema.tables
		WHERE table_schema=DATABASE() ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TableInfo
	for rows.Next() {
		var n, t string
		if err := rows.Scan(&n, &t); err != nil {
			return nil, err
		}
		out = append(out, TableInfo{Name: n, Type: tableType(t)})
	}
	return out, rows.Err()
}

func (mysqlDialect) ListColumns(ctx context.Context, db *sql.DB, _, table string) ([]ColumnInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT column_name, column_type, is_nullable='YES', COALESCE(column_default,''), column_key='PRI'
		FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanColumns(rows)
}

func (mysqlDialect) ListIndexes(ctx context.Context, db *sql.DB, _, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT index_name, column_name, non_unique=0
		FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=?
		ORDER BY index_name, seq_in_index`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndexes(rows)
}
