package driver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	// URI con la ruta escapada: espacios, '#', '?' o '%' en el nombre no rompen el DSN.
	path := filepath.ToSlash(p.FilePath)
	if len(path) >= 2 && path[1] == ':' { // C:\datos\x.db → /C:/datos/x.db
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"}
	return u.String()
}

// Open comprueba que el archivo exista: sin esta comprobación SQLite crearía
// en silencio una base vacía ante cualquier error de ruta.
func (d sqlite) Open(p config.ConnectionProfile, password, database string) (*sql.DB, error) {
	if p.FilePath == "" {
		return nil, errors.New("indica el archivo de la base de datos SQLite")
	}
	st, err := os.Stat(p.FilePath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("el archivo no existe: %s", p.FilePath)
	case err != nil:
		return nil, err
	case st.IsDir():
		return nil, fmt.Errorf("%s es una carpeta, no un archivo", p.FilePath)
	}
	return sql.Open(d.DriverName(), d.DSN(p, password, database))
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
	pks := 0
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		if pk > 0 {
			pks++
		}
		out = append(out, ColumnInfo{Name: name, Type: typ, Nullable: notnull == 0, PrimaryKey: pk > 0, Default: dflt.String})
	}
	// Una clave primaria INTEGER única es un alias del rowid: el motor genera el valor.
	for i := range out {
		out[i].AutoIncrement = pks == 1 && out[i].PrimaryKey && strings.EqualFold(out[i].Type, "INTEGER")
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
		out = append(out, IndexInfo{Name: name, Unique: unique == 1, Primary: origin == "pk"})
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

func (sqlite) Quote(ident string) string { return quoteWith(`"`, `"`, ident) }
func (sqlite) Param(n int) string        { return "?" }
func (sqlite) PageSQL(from, where, order string, limit, offset int) (string, int) {
	return limitOffsetSQL(from, where, order, limit, offset)
}

func (sqlite) ListAllColumns(ctx context.Context, db *sql.DB) ([]ColumnRef, error) {
	// pragma_table_info(...) como función de tabla: una sola consulta en vez de un PRAGMA por tabla.
	rows, err := db.QueryContext(ctx, `
		SELECT '', m.name, m.type, p.name, p.type
		FROM sqlite_master m JOIN pragma_table_info(m.name) p
		WHERE m.type IN ('table','view') AND m.name NOT LIKE 'sqlite\_%' ESCAPE '\'
		ORDER BY m.name, p.cid`)
	if err != nil {
		return nil, err
	}
	return scanColumnRefs(rows, maxAllColumns)
}

func (sqlite) ListForeignKeys(ctx context.Context, db *sql.DB, _, table string) ([]ForeignKeyInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rs []fkRow
	for rows.Next() {
		var id, seq int
		var refTable, from, onUpdate, onDelete, match string
		var to sql.NullString
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		// SQLite no nombra las claves foráneas: se numera por su id.
		rs = append(rs, fkRow{name: fmt.Sprintf("fk_%d", id), refTable: refTable, column: from, refColumn: to.String, onUpdate: fkAction(onUpdate), onDelete: fkAction(onDelete)})
	}
	return groupForeignKeys(rs), rows.Err()
}

// TableDDL devuelve el SQL original guardado por SQLite (tabla o vista, más sus índices).
func (sqlite) TableDDL(ctx context.Context, db *sql.DB, _, table string, _ bool) (string, error) {
	out, err := queryStrings(ctx, db, `SELECT sql FROM sqlite_master WHERE tbl_name = ? AND sql IS NOT NULL
		ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'view' THEN 0 ELSE 1 END, name`, table)
	if err != nil {
		return "", err
	}
	return strings.Join(out, ";\n\n") + ";", nil
}
