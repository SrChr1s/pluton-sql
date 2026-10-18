package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

type sqlite struct{}

func init() { register(sqlite{}) }

func (sqlite) Name() string       { return "sqlite" }
func (sqlite) DriverName() string { return "sqlite" }
func (sqlite) DefaultPort() int   { return 0 }

func (sqlite) DSN(p config.ConnectionProfile, _, _ string) string {
	if p.FilePath == "" {
		return ":memory:"
	}
	return "file:" + p.FilePath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

// SQLite: una sola base ("main") y sin esquemas.
func (sqlite) ListDatabases(context.Context, *sql.DB) ([]string, error) { return []string{"main"}, nil }
func (sqlite) ListSchemas(context.Context, *sql.DB) ([]string, error)   { return []string{""}, nil }

func (sqlite) ListTables(ctx context.Context, db *sql.DB, _ string) ([]TableInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, type FROM sqlite_master
		WHERE type IN ('table','view') AND name NOT LIKE 'sqlite\_%' ESCAPE '\' ORDER BY name`)
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
		out = append(out, TableInfo{Name: n, Type: t})
	}
	return out, rows.Err()
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func (sqlite) ListColumns(ctx context.Context, db *sql.DB, _, table string) ([]ColumnInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ColumnInfo
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out = append(out, ColumnInfo{Name: name, Type: typ, Nullable: notnull == 0, PrimaryKey: pk > 0, Default: dflt.String})
	}
	return out, rows.Err()
}

func (sqlite) ListIndexes(ctx context.Context, db *sql.DB, _, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_list(%s)", quoteIdent(table)))
	if err != nil {
		return nil, err
	}
	var out []IndexInfo
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, IndexInfo{Name: name, Unique: unique == 1})
	}
	rows.Close()
	for i := range out {
		cols, err := queryColumnNames(ctx, db, out[i].Name)
		if err != nil {
			return nil, err
		}
		out[i].Columns = cols
	}
	return out, nil
}

func queryColumnNames(ctx context.Context, db *sql.DB, index string) ([]string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_info(%s)", quoteIdent(index)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		out = append(out, name.String)
	}
	return out, rows.Err()
}
