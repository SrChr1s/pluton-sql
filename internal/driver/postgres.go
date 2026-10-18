package driver

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

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
	// pg_catalog en vez de information_schema: format_type devuelve el tipo completo (varchar(50), numeric(10,2)).
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname, format_type(a.atttypid, a.atttypmod), NOT a.attnotnull,
		  COALESCE(pg_get_expr(d.adbin, d.adrelid), ''),
		  EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = c.oid AND i.indisprimary AND a.attnum = ANY (i.indkey)),
		  a.attidentity <> '' OR COALESCE(pg_get_expr(d.adbin, d.adrelid), '') LIKE 'nextval(%'
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanColumns(rows)
}

func (postgres) ListIndexes(ctx context.Context, db *sql.DB, schema, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT i.relname, a.attname, ix.indisunique, ix.indisprimary
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
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.PrimaryKey, &c.AutoIncrement); err != nil {
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
		if err := rows.Scan(&r.name, &r.column, &r.unique, &r.primary); err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}
	return groupIndexes(rs), rows.Err()
}

func (postgres) Quote(ident string) string { return quoteWith(`"`, `"`, ident) }
func (postgres) Param(n int) string        { return fmt.Sprintf("$%d", n) }
func (postgres) PageSQL(from, where, order string, limit, offset int) (string, int) {
	return limitOffsetSQL(from, where, order, limit, offset)
}

func (postgres) ListForeignKeys(ctx context.Context, db *sql.DB, schema, table string) ([]ForeignKeyInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT con.conname, ra.attname, rn.nspname, rc.relname, fa.attname, con.confupdtype, con.confdeltype
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_class rc ON rc.oid = con.confrelid
		JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS k(src, dst, ord) ON true
		JOIN pg_attribute ra ON ra.attrelid = con.conrelid AND ra.attnum = k.src
		JOIN pg_attribute fa ON fa.attrelid = con.confrelid AND fa.attnum = k.dst
		WHERE con.contype = 'f' AND n.nspname = $1 AND c.relname = $2
		ORDER BY con.conname, k.ord`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	action := map[string]string{"a": "NO ACTION", "r": "RESTRICT", "c": "CASCADE", "n": "SET NULL", "d": "SET DEFAULT"}
	var rs []fkRow
	for rows.Next() {
		var r fkRow
		var up, del string
		if err := rows.Scan(&r.name, &r.column, &r.refSchema, &r.refTable, &r.refColumn, &up, &del); err != nil {
			return nil, err
		}
		r.onUpdate, r.onDelete = action[up], action[del]
		rs = append(rs, r)
	}
	return groupForeignKeys(rs), rows.Err()
}

// TableDDL reconstruye el SQL desde el catálogo (PostgreSQL no tiene un SHOW CREATE TABLE).
func (p postgres) TableDDL(ctx context.Context, db *sql.DB, schema, table string, isView bool) (string, error) {
	qn := p.Quote(schema) + "." + p.Quote(table)
	var oid uint32
	var kind string
	if err := db.QueryRowContext(ctx, `SELECT c.oid::int8, c.relkind::text FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2`, schema, table).Scan(&oid, &kind); err != nil {
		return "", err
	}

	if kind == "v" || kind == "m" {
		var def string
		if err := db.QueryRowContext(ctx, `SELECT pg_get_viewdef($1::oid, true)`, oid).Scan(&def); err != nil {
			return "", err
		}
		head := "CREATE OR REPLACE VIEW "
		if kind == "m" {
			head = "CREATE MATERIALIZED VIEW "
		}
		return head + qn + " AS\n" + strings.TrimSpace(def), nil
	}

	var lines []string
	cols, err := db.QueryContext(ctx, `
		SELECT a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull, a.attidentity::text, a.attgenerated::text,
		       COALESCE(pg_get_expr(d.adbin, d.adrelid), ''),
		       pg_get_serial_sequence($2, a.attname) IS NOT NULL
		FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE a.attrelid = $1::oid AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum`, oid, qn)
	if err != nil {
		return "", err
	}
	defer cols.Close()
	for cols.Next() {
		var name, typ, ident, gen, def string
		var notNull, ownsSeq bool
		if err := cols.Scan(&name, &typ, &notNull, &ident, &gen, &def, &ownsSeq); err != nil {
			return "", err
		}
		// serial/bigserial crean su propia secuencia: copiar "DEFAULT nextval(...)" fallaría al recrear la tabla.
		if serial, ok := map[string]string{"integer": "serial", "bigint": "bigserial", "smallint": "smallserial"}[typ]; ok &&
			ident == "" && ownsSeq && strings.HasPrefix(def, "nextval(") {
			typ, def = serial, ""
		}
		l := "    " + p.Quote(name) + " " + typ
		switch {
		case ident == "a":
			l += " GENERATED ALWAYS AS IDENTITY"
		case ident == "d":
			l += " GENERATED BY DEFAULT AS IDENTITY"
		case gen == "s":
			l += " GENERATED ALWAYS AS (" + def + ") STORED"
		case def != "":
			l += " DEFAULT " + def
		}
		if notNull {
			l += " NOT NULL"
		}
		lines = append(lines, l)
	}
	if err := cols.Err(); err != nil {
		return "", err
	}

	cons, err := db.QueryContext(ctx, `SELECT conname, pg_get_constraintdef(oid, true) FROM pg_constraint WHERE conrelid = $1::oid
		ORDER BY CASE contype WHEN 'p' THEN 1 WHEN 'u' THEN 2 WHEN 'c' THEN 3 ELSE 4 END, conname`, oid)
	if err != nil {
		return "", err
	}
	defer cons.Close()
	for cons.Next() {
		var name, def string
		if err := cons.Scan(&name, &def); err != nil {
			return "", err
		}
		lines = append(lines, "    CONSTRAINT "+p.Quote(name)+" "+def)
	}
	if err := cons.Err(); err != nil {
		return "", err
	}

	out := "CREATE TABLE " + qn + " (\n" + strings.Join(lines, ",\n") + "\n);"

	// Índices que no respaldan una restricción (esos ya van dentro de CREATE TABLE).
	idx, err := db.QueryContext(ctx, `SELECT pg_get_indexdef(i.indexrelid) FROM pg_index i
		WHERE i.indrelid = $1::oid AND NOT EXISTS (SELECT 1 FROM pg_constraint c WHERE c.conindid = i.indexrelid)
		ORDER BY i.indexrelid`, oid)
	if err != nil {
		return "", err
	}
	defer idx.Close()
	for idx.Next() {
		var def string
		if err := idx.Scan(&def); err != nil {
			return "", err
		}
		out += "\n\n" + def + ";"
	}
	return out, idx.Err()
}
