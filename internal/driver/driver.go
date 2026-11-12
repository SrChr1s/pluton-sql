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
	// AutoIncrement: el motor genera el valor (AUTO_INCREMENT, IDENTITY, serial, rowid).
	AutoIncrement bool `json:"autoIncrement"`
	// Comment es el comentario de la columna (PostgreSQL: COMMENT ON; MySQL: COMMENT; SQL Server: MS_Description).
	// SQLite no tiene comentarios y siempre lo deja vacío.
	Comment string `json:"comment"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
}

// ColumnRef sitúa una columna dentro de su tabla y esquema.
type ColumnRef struct {
	Schema   string `json:"schema"` // "" en MySQL y SQLite, que no tienen nivel de esquema
	Table    string `json:"table"`
	IsView   bool   `json:"isView"`
	Column   string `json:"column"`
	DataType string `json:"dataType"`
}

// scanColumnRefs lee filas (esquema, tabla, tipo de objeto, columna, tipo de dato).
func scanColumnRefs(rows *sql.Rows, limit int) ([]ColumnRef, error) {
	defer rows.Close()
	var out []ColumnRef
	for rows.Next() {
		var c ColumnRef
		var kind string
		if err := rows.Scan(&c.Schema, &c.Table, &kind, &c.Column, &c.DataType); err != nil {
			return nil, err
		}
		c.IsView = strings.EqualFold(kind, "VIEW")
		out = append(out, c)
		if limit > 0 && len(out) >= limit {
			break // base enorme: el autocompletado se conforma con una parte
		}
	}
	return out, rows.Err()
}

// maxAllColumns acota el índice de autocompletado en bases con decenas de miles de columnas.
const maxAllColumns = 100000

// ForeignKeyInfo describe una clave foránea saliente de una tabla.
type ForeignKeyInfo struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefSchema  string   `json:"refSchema"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnUpdate   string   `json:"onUpdate"`
	OnDelete   string   `json:"onDelete"`
}

// TableScript es el DDL de una tabla separado en piezas, para poder ordenar un script de varias tablas:
// primero todos los CREATE TABLE, después los datos y al final índices y claves foráneas.
type TableScript struct {
	Create      string   `json:"create"`      // CREATE TABLE sin claves foráneas (salvo MySQL, que las deja dentro)
	Indexes     []string `json:"indexes"`     // CREATE INDEX que no respaldan una restricción
	ForeignKeys []string `json:"foreignKeys"` // ALTER TABLE … ADD CONSTRAINT … FOREIGN KEY
	// SkipColumns son las columnas que no se pueden insertar: calculadas, rowversion, GENERATED … STORED.
	SkipColumns []string `json:"skipColumns"`
	// Identity: la tabla tiene una columna que el motor genera y que al insertar hay que forzar
	// (SQL Server: IDENTITY_INSERT; PostgreSQL: OVERRIDING SYSTEM VALUE).
	Identity bool `json:"identity"`
	// SeqColumns son las columnas serial/identity de PostgreSQL, cuya secuencia hay que reajustar tras insertar datos.
	SeqColumns []string `json:"seqColumns"`
	// Comments son las sentencias que fijan los comentarios de columna, para ejecutar tras el CREATE TABLE.
	// Vacío en MySQL (el COMMENT va dentro del CREATE) y en SQLite.
	Comments []string `json:"comments"`
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

	// ListAllColumns devuelve todas las columnas de todas las tablas y vistas de la base conectada en una
	// sola consulta (para el autocompletado), ordenadas por esquema, tabla y posición.
	ListAllColumns(ctx context.Context, db *sql.DB) ([]ColumnRef, error)
	ListForeignKeys(ctx context.Context, db *sql.DB, schema, table string) ([]ForeignKeyInfo, error)
	// TableDDL devuelve el SQL que crea la tabla o vista (con sus índices y claves).
	TableDDL(ctx context.Context, db *sql.DB, schema, table string, isView bool) (string, error)
	// TableScript devuelve el DDL de una tabla (no vista) por piezas; ver TableScript.
	TableScript(ctx context.Context, db *sql.DB, schema, table string) (TableScript, error)

	// ListRoutines lista procedimientos y funciones de un esquema (MySQL: de la base conectada; SQLite: ninguno).
	ListRoutines(ctx context.Context, db *sql.DB, schema string) ([]RoutineInfo, error)
	// RoutineDDL devuelve el SQL que crea la rutina.
	RoutineDDL(ctx context.Context, db *sql.DB, schema string, r RoutineInfo) (string, error)

	// Quote cita un identificador (tabla, columna...) escapando las comillas.
	Quote(ident string) string
	// Param devuelve el marcador del parámetro n (1-based): $1, ?, @p1.
	Param(n int) string
	// PageSQL devuelve la consulta de una página de from (tabla ya citada). where y order son
	// cláusulas ya construidas (o ""). hidden es el número de columnas auxiliares al principio
	// del resultado que el llamador debe descartar.
	PageSQL(from, where, order string, limit, offset int) (query string, hidden int)
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
	name, column    string
	unique, primary bool
}

func groupIndexes(rows []indexRow) []IndexInfo {
	var out []IndexInfo
	pos := map[string]int{}
	for _, r := range rows {
		i, ok := pos[r.name]
		if !ok {
			i = len(out)
			pos[r.name] = i
			out = append(out, IndexInfo{Name: r.name, Unique: r.unique, Primary: r.primary})
		}
		out[i].Columns = append(out[i].Columns, r.column)
	}
	return out
}

func quoteWith(open, close, ident string) string {
	return open + strings.ReplaceAll(ident, close, close+close) + close
}

// limitOffsetSQL es la paginación estándar de PostgreSQL, MySQL y SQLite.
func limitOffsetSQL(from, where, order string, limit, offset int) (string, int) {
	if order != "" {
		order += " "
	}
	return fmt.Sprintf("SELECT * FROM %s%s %sLIMIT %d OFFSET %d", from, where, order, limit, offset), 0
}

// fkRow es una fila (restricción, columna origen, destino) antes de agrupar.
type fkRow struct {
	name, refSchema, refTable, column, refColumn, onUpdate, onDelete string
}

func groupForeignKeys(rows []fkRow) []ForeignKeyInfo {
	var out []ForeignKeyInfo
	pos := map[string]int{}
	for _, r := range rows {
		i, ok := pos[r.name]
		if !ok {
			i = len(out)
			pos[r.name] = i
			out = append(out, ForeignKeyInfo{Name: r.name, RefSchema: r.refSchema, RefTable: r.refTable, OnUpdate: r.onUpdate, OnDelete: r.onDelete})
		}
		out[i].Columns = append(out[i].Columns, r.column)
		out[i].RefColumns = append(out[i].RefColumns, r.refColumn)
	}
	return out
}

// fkAction normaliza la acción referencial a "NO ACTION", "CASCADE", "SET NULL"...
func fkAction(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), "_", " "))
}
