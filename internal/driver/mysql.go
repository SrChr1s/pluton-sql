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
	// RowsAffected cuenta filas coincidentes y no solo modificadas: un UPDATE que deja
	// el mismo valor devolvería 0 y se tomaría por fila no encontrada.
	cfg.ClientFoundRows = true
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
	rows, err := db.QueryContext(ctx, `SELECT column_name, column_type, is_nullable='YES', COALESCE(NULLIF(column_default,'NULL'),''), column_key='PRI', extra LIKE '%auto_increment%'
		FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanColumns(rows)
}

func (mysqlDialect) ListIndexes(ctx context.Context, db *sql.DB, _, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT index_name, column_name, non_unique=0, index_name='PRIMARY'
		FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=?
		ORDER BY index_name, seq_in_index`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndexes(rows)
}

func (mysqlDialect) Quote(ident string) string { return quoteWith("`", "`", ident) }
func (mysqlDialect) Param(n int) string        { return "?" }
func (mysqlDialect) PageSQL(from, where, order string, limit, offset int) (string, int) {
	return limitOffsetSQL(from, where, order, limit, offset)
}

func (mysqlDialect) ListAllColumns(ctx context.Context, db *sql.DB) ([]ColumnRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT '', c.table_name, t.table_type, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = DATABASE()
		ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		return nil, err
	}
	return scanColumnRefs(rows, maxAllColumns)
}

func (mysqlDialect) ListForeignKeys(ctx context.Context, db *sql.DB, _, table string) ([]ForeignKeyInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT k.constraint_name, k.column_name, k.referenced_table_schema, k.referenced_table_name, k.referenced_column_name,
		       r.update_rule, r.delete_rule
		FROM information_schema.key_column_usage k
		JOIN information_schema.referential_constraints r
		  ON r.constraint_schema = k.constraint_schema AND r.constraint_name = k.constraint_name AND r.table_name = k.table_name
		WHERE k.table_schema = DATABASE() AND k.table_name = ? AND k.referenced_table_name IS NOT NULL
		ORDER BY k.constraint_name, k.ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rs []fkRow
	for rows.Next() {
		var r fkRow
		if err := rows.Scan(&r.name, &r.column, &r.refSchema, &r.refTable, &r.refColumn, &r.onUpdate, &r.onDelete); err != nil {
			return nil, err
		}
		r.onUpdate, r.onDelete = fkAction(r.onUpdate), fkAction(r.onDelete)
		rs = append(rs, r)
	}
	return groupForeignKeys(rs), rows.Err()
}

// TableDDL usa SHOW CREATE: la segunda columna es el SQL (tablas y vistas devuelven distinto número de columnas).
func (d mysqlDialect) TableDDL(ctx context.Context, db *sql.DB, _, table string, isView bool) (string, error) {
	kind := "TABLE"
	if isView {
		kind = "VIEW"
	}
	rows, err := db.QueryContext(ctx, "SHOW CREATE "+kind+" "+d.Quote(table))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", rows.Err()
	}
	vals := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	if len(vals) < 2 {
		return "", fmt.Errorf("SHOW CREATE %s devolvió %d columnas", kind, len(vals))
	}
	return string(vals[1]) + ";", nil
}
