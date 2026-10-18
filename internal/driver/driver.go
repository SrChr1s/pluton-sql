// Package driver aísla las diferencias entre motores SQL.
package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

type TableInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
	Type   string `json:"type"` // table | view
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primaryKey"`
	Default    string `json:"default"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// Dialect encapsula DSN y consultas de catálogo de un motor.
//
// Cada pool *sql.DB está conectado a una sola base de datos, por lo que
// ListSchemas/ListTables operan sobre la base conectada. MySQL y SQLite no
// tienen nivel de esquema: ListSchemas devuelve un único "" implícito.
type Dialect interface {
	Name() string
	DriverName() string
	DefaultPort() int
	// DSN construye la cadena de conexión; database vacío usa la del perfil.
	DSN(p config.ConnectionProfile, password, database string) string
	ListDatabases(ctx context.Context, db *sql.DB) ([]string, error)
	ListSchemas(ctx context.Context, db *sql.DB) ([]string, error)
	ListTables(ctx context.Context, db *sql.DB, schema string) ([]TableInfo, error)
	ListColumns(ctx context.Context, db *sql.DB, schema, table string) ([]ColumnInfo, error)
	ListIndexes(ctx context.Context, db *sql.DB, schema, table string) ([]IndexInfo, error)

	// Quote cita un identificador (tabla, columna...) escapando las comillas.
	Quote(ident string) string
	// Param devuelve el marcador del parámetro n (1-based): $1, ?, @p1.
	Param(n int) string
	// Paginate devuelve el sufijo de paginación. order es una cláusula ORDER BY
	// ya construida o "" si no hay orden.
	Paginate(order string, limit, offset int) string
}

// Opener es opcional: un dialecto lo implementa cuando la cadena DSN no basta
// para configurar la conexión (p. ej. TLS personalizado en PostgreSQL).
type Opener interface {
	Open(p config.ConnectionProfile, password, database string) (*sql.DB, error)
}

var registry = map[string]Dialect{}

func register(d Dialect) { registry[d.Name()] = d }

// Get devuelve el dialecto por nombre (postgres, mysql, sqlite, mssql).
func Get(name string) (Dialect, error) {
	d, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("motor no soportado: %q", name)
	}
	return d, nil
}

// Names lista los motores registrados.
func Names() []string {
	return []string{"postgres", "mysql", "sqlite", "mssql"}
}

func pick(database, fallback string) string {
	if database != "" {
		return database
	}
	return fallback
}

// queryStrings ejecuta q y devuelve la primera columna como []string.
func queryStrings(ctx context.Context, db *sql.DB, q string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// groupIndexes agrupa filas (index, column, unique) preservando el orden.
type indexRow struct {
	name, column string
	unique       bool
}

func groupIndexes(rows []indexRow) []IndexInfo {
	var out []IndexInfo
	pos := map[string]int{}
	for _, r := range rows {
		i, ok := pos[r.name]
		if !ok {
			i = len(out)
			pos[r.name] = i
			out = append(out, IndexInfo{Name: r.name, Unique: r.unique})
		}
		out[i].Columns = append(out[i].Columns, r.column)
	}
	return out
}

func quoteWith(open, close, ident string) string {
	return open + strings.ReplaceAll(ident, close, close+close) + close
}

// limitOffset es la paginación estándar de PostgreSQL, MySQL y SQLite.
func limitOffset(order string, limit, offset int) string {
	if order != "" {
		order += " "
	}
	return fmt.Sprintf("%sLIMIT %d OFFSET %d", order, limit, offset)
}
