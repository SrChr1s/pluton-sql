package driver

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	mssqldb "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

type mssql struct{}

func init() {
	register(mssql{})
	// PLUTON_DEBUG=1 vuelca a stderr la traza del protocolo TDS (diagnóstico de conexión).
	if os.Getenv("PLUTON_DEBUG") != "" {
		mssqldb.SetLogger(log.New(os.Stderr, "[mssql] ", log.Ltime|log.Lmicroseconds))
		log.SetFlags(log.Ltime | log.Lmicroseconds)
	}
}

func (mssql) Name() string       { return "mssql" }
func (mssql) DriverName() string { return "sqlserver" }
func (mssql) DefaultPort() int   { return 1433 }

func (mssql) DSN(p config.ConnectionProfile, password, database string) string {
	port := p.Port
	if port == 0 {
		port = 1433
	}
	q := url.Values{}
	if db := pick(database, p.Database); db != "" {
		q.Set("database", db)
	}
	q.Set("encrypt", pick(p.SSLMode, "true"))
	q.Set("TrustServerCertificate", "true")
	// SQL Server anterior a 2012 corta la conexión si el prelogin incluye el campo TRACEID.
	q.Set("notraceid", "true")
	if v := legacyTLS(p.TLSMinVersion); v != "" {
		q.Set("tlsmin", v)
	}
	if os.Getenv("PLUTON_DEBUG") != "" {
		q.Set("log", "67") // errores + mensajes + depuración
	}
	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(p.User, password),
		Host:     fmt.Sprintf("%s:%d", p.Host, port),
		RawQuery: q.Encode(),
	}
	return u.String()
}

// Open construye el conector; con PLUTON_DEBUG añade el registro TCP (spyDialer).
func (d mssql) Open(p config.ConnectionProfile, password, database string) (*sql.DB, error) {
	cfg, err := msdsn.Parse(d.DSN(p, password, database))
	if err != nil {
		return nil, err
	}
	conn := mssqldb.NewConnectorConfig(cfg)
	if os.Getenv("PLUTON_DEBUG") != "" {
		conn.Dialer = spyDialer{}
	}
	return sql.OpenDB(conn), nil
}

func (mssql) ListDatabases(ctx context.Context, db *sql.DB) ([]string, error) {
	return queryStrings(ctx, db, `SELECT name FROM sys.databases WHERE database_id > 4 AND state = 0 ORDER BY name`)
}

func (mssql) ListSchemas(ctx context.Context, db *sql.DB) ([]string, error) {
	return queryStrings(ctx, db, `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('sys','INFORMATION_SCHEMA','guest') AND schema_name NOT LIKE 'db[_]%' ORDER BY schema_name`)
}

func (mssql) ListTables(ctx context.Context, db *sql.DB, schema string) ([]TableInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT table_name, table_type FROM information_schema.tables
		WHERE table_schema=@p1 ORDER BY table_name`, schema)
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

func (mssql) ListColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.column_name,
		  CASE WHEN c.data_type IN ('char','varchar','nchar','nvarchar','binary','varbinary')
		         THEN c.data_type + '(' + CASE WHEN c.character_maximum_length = -1 THEN 'max' ELSE CAST(c.character_maximum_length AS varchar(10)) END + ')'
		       WHEN c.data_type IN ('decimal','numeric')
		         THEN c.data_type + '(' + CAST(c.numeric_precision AS varchar(5)) + ',' + CAST(c.numeric_scale AS varchar(5)) + ')'
		       ELSE c.data_type END,
		  CASE WHEN c.is_nullable='YES' THEN 1 ELSE 0 END, ISNULL(c.column_default,''),
		  CASE WHEN EXISTS (SELECT 1 FROM information_schema.table_constraints tc
		    JOIN information_schema.key_column_usage k ON k.constraint_name=tc.constraint_name AND k.table_schema=tc.table_schema
		    WHERE tc.constraint_type='PRIMARY KEY' AND tc.table_schema=c.table_schema AND tc.table_name=c.table_name AND k.column_name=c.column_name)
		  THEN 1 ELSE 0 END,
		  CASE WHEN COLUMNPROPERTY(OBJECT_ID(QUOTENAME(c.table_schema) + '.' + QUOTENAME(c.table_name)), c.column_name, 'IsIdentity') = 1 THEN 1 ELSE 0 END
		FROM information_schema.columns c
		WHERE c.table_schema=@p1 AND c.table_name=@p2 ORDER BY c.ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanColumns(rows)
}

func (mssql) ListIndexes(ctx context.Context, db *sql.DB, schema, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT i.name, c.name, i.is_unique, i.is_primary_key
		FROM sys.indexes i
		JOIN sys.index_columns ic ON ic.object_id=i.object_id AND ic.index_id=i.index_id
		JOIN sys.columns c ON c.object_id=ic.object_id AND c.column_id=ic.column_id
		JOIN sys.tables t ON t.object_id=i.object_id
		JOIN sys.schemas s ON s.schema_id=t.schema_id
		WHERE s.name=@p1 AND t.name=@p2 AND i.name IS NOT NULL
		ORDER BY i.name, ic.key_ordinal`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndexes(rows)
}

func (mssql) Quote(ident string) string { return quoteWith("[", "]", ident) }
func (mssql) Param(n int) string        { return fmt.Sprintf("@p%d", n) }

// PageSQL usa ROW_NUMBER() y no OFFSET/FETCH: este último solo existe desde SQL Server 2012.
func (mssql) PageSQL(from, where, order string, limit, offset int) (string, int) {
	if order == "" {
		order = "ORDER BY (SELECT NULL)"
	}
	return fmt.Sprintf("SELECT * FROM (SELECT ROW_NUMBER() OVER (%s) AS __rn, * FROM %s%s) AS __p WHERE __rn > %d AND __rn <= %d ORDER BY __rn",
		order, from, where, offset, offset+limit), 1
}

func (mssql) ListForeignKeys(ctx context.Context, db *sql.DB, schema, table string) ([]ForeignKeyInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT fk.name, pc.name, rs.name, rt.name, rc.name, fk.update_referential_action_desc, fk.delete_referential_action_desc
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
		JOIN sys.tables pt ON pt.object_id = fk.parent_object_id
		JOIN sys.schemas ps ON ps.schema_id = pt.schema_id
		JOIN sys.columns pc ON pc.object_id = fkc.parent_object_id AND pc.column_id = fkc.parent_column_id
		JOIN sys.tables rt ON rt.object_id = fk.referenced_object_id
		JOIN sys.schemas rs ON rs.schema_id = rt.schema_id
		JOIN sys.columns rc ON rc.object_id = fkc.referenced_object_id AND rc.column_id = fkc.referenced_column_id
		WHERE ps.name = @p1 AND pt.name = @p2
		ORDER BY fk.name, fkc.constraint_column_id`, schema, table)
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

// TableDDL: las vistas se leen tal cual; las tablas se reconstruyen desde el catálogo
// (SQL Server no tiene SHOW CREATE), por lo que puede omitir CHECK, triggers y opciones de almacenamiento.
func (m mssql) TableDDL(ctx context.Context, db *sql.DB, schema, table string, isView bool) (string, error) {
	qn := m.Quote(schema) + "." + m.Quote(table)
	if isView {
		var def sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT OBJECT_DEFINITION(OBJECT_ID(@p1))`, qn).Scan(&def); err != nil {
			return "", err
		}
		return strings.TrimSpace(def.String), nil
	}
	cols, err := m.ListColumns(ctx, db, schema, table)
	if err != nil {
		return "", err
	}
	idxs, err := m.ListIndexes(ctx, db, schema, table)
	if err != nil {
		return "", err
	}
	fks, err := m.ListForeignKeys(ctx, db, schema, table)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, c := range cols {
		l := "    " + m.Quote(c.Name) + " " + c.Type
		if c.AutoIncrement {
			var seed, inc sql.NullInt64
			_ = db.QueryRowContext(ctx, `SELECT CAST(seed_value AS bigint), CAST(increment_value AS bigint) FROM sys.identity_columns
				WHERE object_id = OBJECT_ID(@p1) AND name = @p2`, qn, c.Name).Scan(&seed, &inc)
			if !seed.Valid {
				seed.Int64, inc.Int64 = 1, 1
			}
			l += fmt.Sprintf(" IDENTITY(%d,%d)", seed.Int64, inc.Int64)
		}
		if !c.Nullable {
			l += " NOT NULL"
		}
		if c.Default != "" {
			l += " DEFAULT " + c.Default
		}
		lines = append(lines, l)
	}
	var tail []string
	for _, ix := range idxs {
		q := make([]string, len(ix.Columns))
		for i, c := range ix.Columns {
			q[i] = m.Quote(c)
		}
		switch {
		case ix.Primary:
			lines = append(lines, "    CONSTRAINT "+m.Quote(ix.Name)+" PRIMARY KEY ("+strings.Join(q, ", ")+")")
		default:
			u := ""
			if ix.Unique {
				u = "UNIQUE "
			}
			tail = append(tail, "CREATE "+u+"INDEX "+m.Quote(ix.Name)+" ON "+qn+" ("+strings.Join(q, ", ")+");")
		}
	}
	for _, fk := range fks {
		cs, rs := make([]string, len(fk.Columns)), make([]string, len(fk.RefColumns))
		for i := range fk.Columns {
			cs[i], rs[i] = m.Quote(fk.Columns[i]), m.Quote(fk.RefColumns[i])
		}
		tail = append(tail, fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s.%s (%s) ON UPDATE %s ON DELETE %s;",
			qn, m.Quote(fk.Name), strings.Join(cs, ", "), m.Quote(fk.RefSchema), m.Quote(fk.RefTable), strings.Join(rs, ", "), fk.OnUpdate, fk.OnDelete))
	}
	out := "CREATE TABLE " + qn + " (\n" + strings.Join(lines, ",\n") + "\n);"
	if len(tail) > 0 {
		out += "\n\n" + strings.Join(tail, "\n")
	}
	return out, nil
}
