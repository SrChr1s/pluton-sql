package driver

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

type postgres struct{}

func init() { register(postgres{}) }

func (postgres) Name() string       { return "postgres" }
func (postgres) DriverName() string { return "pgx" }
func (postgres) DefaultPort() int   { return 5432 }

func (postgres) DSN(p config.ConnectionProfile, password, database string) string {
	port := p.Port
	if port == 0 {
		port = 5432
	}
	ssl := pick(p.SSLMode, "prefer")
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(p.User, password),
		Host:     fmt.Sprintf("%s:%d", p.Host, port),
		Path:     "/" + pick(database, pick(p.Database, "postgres")),
		RawQuery: "sslmode=" + url.QueryEscape(ssl),
	}
	return u.String()
}

// Open aplica la versión mínima de TLS sobre la configuración de pgx.
func (d postgres) Open(p config.ConnectionProfile, password, database string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(d.DSN(p, password, database))
	if err != nil {
		return nil, err
	}
	if v := legacyTLS(p.TLSMinVersion); v != "" {
		relax := func(c *tls.Config) {
			if c != nil {
				c.MinVersion = tlsVersions[v]
				c.CipherSuites = allCipherSuites()
			}
		}
		relax(cfg.TLSConfig)
		for _, f := range cfg.Fallbacks {
			relax(f.TLSConfig)
		}
	}
	return stdlib.OpenDB(*cfg), nil
}

func (postgres) ListDatabases(ctx context.Context, db *sql.DB) ([]string, error) {
	return queryStrings(ctx, db, `SELECT datname FROM pg_database WHERE NOT datistemplate AND datallowconn ORDER BY datname`)
}

func (postgres) ListSchemas(ctx context.Context, db *sql.DB) ([]string, error) {
	return queryStrings(ctx, db, `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema','pg_catalog','pg_toast') AND schema_name NOT LIKE 'pg\_temp\_%' AND schema_name NOT LIKE 'pg\_toast\_temp\_%'
		ORDER BY schema_name`)
}

func (postgres) ListTables(ctx context.Context, db *sql.DB, schema string) ([]TableInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT table_name, table_type FROM information_schema.tables
		WHERE table_schema=$1 ORDER BY table_name`, schema)
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
		out = append(out, TableInfo{Name: n, Schema: schema, Type: tableType(t)})
	}
	return out, rows.Err()
}

func (postgres) ListColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.column_name, c.data_type, c.is_nullable='YES', COALESCE(c.column_default,''),
		  EXISTS (SELECT 1 FROM information_schema.table_constraints tc
		    JOIN information_schema.key_column_usage k ON k.constraint_name=tc.constraint_name AND k.table_schema=tc.table_schema
		    WHERE tc.constraint_type='PRIMARY KEY' AND tc.table_schema=c.table_schema AND tc.table_name=c.table_name AND k.column_name=c.column_name)
		FROM information_schema.columns c
		WHERE c.table_schema=$1 AND c.table_name=$2 ORDER BY c.ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanColumns(rows)
}

func (postgres) ListIndexes(ctx context.Context, db *sql.DB, schema, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT i.relname, a.attname, ix.indisunique
		FROM pg_index ix
		JOIN pg_class i ON i.oid=ix.indexrelid
		JOIN pg_class t ON t.oid=ix.indrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
		JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum
		WHERE n.nspname=$1 AND t.relname=$2
		ORDER BY i.relname, k.ord`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndexes(rows)
}

func tableType(t string) string {
	if t == "VIEW" {
		return "view"
	}
	return "table"
}

func scanColumns(rows *sql.Rows) ([]ColumnInfo, error) {
	var out []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.PrimaryKey); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanIndexes(rows *sql.Rows) ([]IndexInfo, error) {
	var rs []indexRow
	for rows.Next() {
		var r indexRow
		if err := rows.Scan(&r.name, &r.column, &r.unique); err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}
	return groupIndexes(rs), rows.Err()
}
