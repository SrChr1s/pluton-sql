package driver

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"

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
		SELECT c.column_name, c.data_type, CASE WHEN c.is_nullable='YES' THEN 1 ELSE 0 END, ISNULL(c.column_default,''),
		  CASE WHEN EXISTS (SELECT 1 FROM information_schema.table_constraints tc
		    JOIN information_schema.key_column_usage k ON k.constraint_name=tc.constraint_name AND k.table_schema=tc.table_schema
		    WHERE tc.constraint_type='PRIMARY KEY' AND tc.table_schema=c.table_schema AND tc.table_name=c.table_name AND k.column_name=c.column_name)
		  THEN 1 ELSE 0 END
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
		SELECT i.name, c.name, i.is_unique
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
func (mssql) Paginate(order string, limit, offset int) string {
	if order == "" {
		order = "ORDER BY (SELECT NULL)" // OFFSET/FETCH exige ORDER BY
	}
	return fmt.Sprintf("%s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", order, offset, limit)
}
