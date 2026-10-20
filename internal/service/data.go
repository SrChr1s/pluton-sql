package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

// Pools entrega el pool y el dialecto de una conexión; lo implementa *session.Manager.
type Pools interface {
	Get(ctx context.Context, profileID, database string) (*sql.DB, driver.Dialect, error)
}

var _ Pools = (*session.Manager)(nil)

const (
	defaultPageSize = 100
	maxPageSize     = 1000
	dataTimeout     = 60 * time.Second
)

type TableRef struct {
	Profile  string `json:"profile"`
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
}

type PageRequest struct {
	Table   TableRef `json:"table"`
	Offset  int      `json:"offset"`
	Limit   int      `json:"limit"`
	OrderBy string   `json:"orderBy"`
	Desc    bool     `json:"desc"`
	Filter  string   `json:"filter"` // expresión WHERE sin la palabra WHERE
}

type TablePage struct {
	Columns []driver.ColumnInfo `json:"columns"`
	Rows    [][]any             `json:"rows"`
	HasMore bool                `json:"hasMore"`
	// KeyColumns identifica una fila: la clave primaria o, sin ella, todas las
	// columnas comparables. Vacío = no se pueden editar ni borrar filas.
	KeyColumns []string `json:"keyColumns"`
	HasPK      bool     `json:"hasPk"`
	IsView     bool     `json:"isView"`
	DurationMs int64    `json:"durationMs"`
}

// Change es una modificación pendiente de una fila.
type Change struct {
	Kind string `json:"kind"` // update | insert | delete
	// Key son los valores originales de la fila (update/delete); basta con enviarlos todos.
	Key map[string]any `json:"key"`
	// Values son las columnas nuevas: solo las modificadas en update, las dadas en insert.
	Values map[string]any `json:"values"`
}

type ApplyResult struct {
	Updated  int `json:"updated"`
	Inserted int `json:"inserted"`
	Deleted  int `json:"deleted"`
}

type DataService struct {
	pools Pools
	ctx   context.Context // de Wails: lo necesitan los diálogos de guardar
}

// SetContext recibe el contexto de Wails, necesario para mostrar diálogos.
func (s *DataService) SetContext(ctx context.Context) { s.ctx = ctx }

func NewDataService(pools Pools) *DataService { return &DataService{pools: pools} }

// Page lee una página de la tabla con orden y filtro opcionales.
func (s *DataService) Page(req PageRequest) (*TablePage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	t, db, d, meta, err := s.open(ctx, req.Table)
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	limit = min(limit, maxPageSize)
	offset := max(req.Offset, 0)

	keys, hasPK := keyColumns(meta, d.Name())
	order, err := orderClause(d, meta, req.OrderBy, req.Desc, keys, hasPK)
	if err != nil {
		return nil, err
	}
	where, err := whereClause(req.Filter)
	if err != nil {
		return nil, err
	}

	// Una fila de más para saber si hay página siguiente sin contar toda la tabla.
	q, hidden := d.PageSQL(qualified(d, req.Table), where, order, limit+1, offset)
	start := time.Now()
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &QueryResult{Columns: []string{}, Rows: [][]any{}}
	if err := readRows(rows, res, limit); err != nil {
		return nil, err
	}
	res.Columns = res.Columns[hidden:] // columnas auxiliares de la paginación (p. ej. __rn en SQL Server)
	for i := range res.Rows {
		res.Rows[i] = res.Rows[i][hidden:]
	}

	byName := map[string]driver.ColumnInfo{}
	for _, c := range meta {
		byName[c.Name] = c
	}
	cols := make([]driver.ColumnInfo, len(res.Columns))
	for i, n := range res.Columns {
		cols[i] = byName[n]
		cols[i].Name = n
	}
	return &TablePage{
		Columns: cols, Rows: res.Rows, HasMore: res.Truncated,
		KeyColumns: keys, HasPK: hasPK, IsView: t.Type == "view",
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// Count cuenta las filas que cumplen el filtro (puede tardar en tablas grandes).
func (s *DataService) Count(ref TableRef, filter string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	_, db, d, _, err := s.open(ctx, ref)
	if err != nil {
		return 0, err
	}
	where, err := whereClause(filter)
	if err != nil {
		return 0, err
	}
	var n int64
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qualified(d, ref)+where).Scan(&n)
	return n, err
}

// Apply ejecuta todos los cambios en una transacción: si uno falla, no se aplica ninguno.
// Cada update/delete debe afectar exactamente una fila; con claves no únicas (tablas sin
// clave primaria) eso evita tocar filas duplicadas por error.
func (s *DataService) Apply(ref TableRef, changes []Change) (*ApplyResult, error) {
	if len(changes) == 0 {
		return &ApplyResult{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	t, db, d, meta, err := s.open(ctx, ref)
	if err != nil {
		return nil, err
	}
	if t.Type == "view" {
		return nil, errors.New("las vistas son de solo lectura")
	}
	keys, _ := keyColumns(meta, d.Name())
	known := map[string]bool{}
	for _, c := range meta {
		known[c.Name] = true
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // no-op tras Commit

	res := &ApplyResult{}
	for i, c := range changes {
		stmt, args, err := buildChange(d, ref, c, keys, known)
		if err != nil {
			return nil, fmt.Errorf("cambio %d (%s): %w", i+1, c.Kind, err)
		}
		r, err := tx.ExecContext(ctx, stmt, args...)
		if err != nil {
			return nil, fmt.Errorf("cambio %d (%s): %w", i+1, c.Kind, err)
		}
		if n, err := r.RowsAffected(); err == nil && n != 1 {
			return nil, fmt.Errorf("cambio %d (%s): se esperaba afectar 1 fila y se afectaron %d "+
				"(la fila cambió desde que se cargó, o no se distingue de otras idénticas); no se aplicó ningún cambio", i+1, c.Kind, n)
		}
		switch c.Kind {
		case "update":
			res.Updated++
		case "insert":
			res.Inserted++
		case "delete":
			res.Deleted++
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

// open resuelve pool, dialecto, metadatos de columnas y tipo (tabla/vista).
func (s *DataService) open(ctx context.Context, ref TableRef) (driver.TableInfo, *sql.DB, driver.Dialect, []driver.ColumnInfo, error) {
	return openTable(ctx, s.pools, ref)
}

func openTable(ctx context.Context, pools Pools, ref TableRef) (driver.TableInfo, *sql.DB, driver.Dialect, []driver.ColumnInfo, error) {
	var t driver.TableInfo
	if ref.Table == "" {
		return t, nil, nil, nil, errors.New("falta el nombre de la tabla")
	}
	db, d, err := pools.Get(ctx, ref.Profile, ref.Database)
	if err != nil {
		return t, nil, nil, nil, err
	}
	meta, err := d.ListColumns(ctx, db, ref.Schema, ref.Table)
	if err != nil {
		return t, nil, nil, nil, err
	}
	if len(meta) == 0 {
		return t, nil, nil, nil, fmt.Errorf("la tabla %q no existe o no tiene columnas", ref.Table)
	}
	t = driver.TableInfo{Name: ref.Table, Schema: ref.Schema, Type: "table"}
	if tables, err := d.ListTables(ctx, db, ref.Schema); err == nil {
		for _, x := range tables {
			if x.Name == ref.Table {
				t = x
			}
		}
	}
	return t, db, d, meta, nil
}

func qualified(d driver.Dialect, ref TableRef) string {
	if ref.Schema == "" {
		return d.Quote(ref.Table)
	}
	return d.Quote(ref.Schema) + "." + d.Quote(ref.Table)
}

// Tipos que no admiten comparación con = (o cuyo valor no viaja intacto por JSON).
var (
	uncomparable    = regexp.MustCompile(`(?i)(blob|binary|bytea|image|xml|json|geometry|geography|point|polygon|ntext)`)
	uncomparableTxt = regexp.MustCompile(`(?i)^text$`)
)

// keyColumns devuelve la clave primaria o, sin ella, todas las columnas comparables.
func keyColumns(meta []driver.ColumnInfo, driverName string) (cols []string, hasPK bool) {
	for _, c := range meta {
		if c.PrimaryKey {
			cols = append(cols, c.Name)
		}
	}
	if len(cols) > 0 {
		return cols, true
	}
	for _, c := range meta {
		if uncomparable.MatchString(c.Type) || (driverName == "mssql" && uncomparableTxt.MatchString(c.Type)) {
			continue
		}
		cols = append(cols, c.Name)
	}
	return cols, false
}

func orderClause(d driver.Dialect, meta []driver.ColumnInfo, orderBy string, desc bool, keys []string, hasPK bool) (string, error) {
	if orderBy != "" {
		for _, c := range meta {
			if c.Name == orderBy {
				dir := "ASC"
				if desc {
					dir = "DESC"
				}
				return "ORDER BY " + d.Quote(orderBy) + " " + dir, nil
			}
		}
		return "", fmt.Errorf("no se puede ordenar: la columna %q no existe", orderBy)
	}
	if hasPK { // orden estable para que la paginación no repita ni se salte filas
		q := make([]string, len(keys))
		for i, k := range keys {
			q[i] = d.Quote(k)
		}
		return "ORDER BY " + strings.Join(q, ", "), nil
	}
	return "", nil
}

func whereClause(filter string) (string, error) {
	f := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(filter), ";"))
	if f == "" {
		return "", nil
	}
	if strings.Contains(f, ";") {
		return "", errors.New("el filtro debe ser una sola expresión, sin ';'")
	}
	return " WHERE (" + f + ")", nil
}

// coerce deja los números enteros como int64: JSON los entrega como float64 y algunos
// drivers rechazan un float64 para una columna entera.
func coerce(v any) any {
	if f, ok := v.(float64); ok && f == float64(int64(f)) && f > -(1<<53) && f < 1<<53 {
		return int64(f)
	}
	return v
}

// keyWhere arma la condición que identifica una fila con los valores originales de sus columnas clave.
func keyWhere(d driver.Dialect, keys []string, key map[string]any, param func(any) string) (string, error) {
	if len(keys) == 0 {
		return "", errors.New("la tabla no tiene columnas con las que identificar una fila")
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v, ok := key[k]
		if !ok {
			return "", fmt.Errorf("falta el valor original de la columna %q", k)
		}
		if v == nil {
			parts = append(parts, d.Quote(k)+" IS NULL")
		} else {
			parts = append(parts, d.Quote(k)+" = "+param(v))
		}
	}
	return strings.Join(parts, " AND "), nil
}

func buildChange(d driver.Dialect, ref TableRef, c Change, keys []string, known map[string]bool) (string, []any, error) {
	tbl := qualified(d, ref)
	var args []any
	param := func(v any) string {
		args = append(args, coerce(v))
		return d.Param(len(args))
	}
	valueCols := func() ([]string, error) {
		names := make([]string, 0, len(c.Values))
		for n := range c.Values {
			if !known[n] {
				return nil, fmt.Errorf("la columna %q no existe", n)
			}
			names = append(names, n)
		}
		sortStrings(names)
		return names, nil
	}
	where := func() (string, error) { return keyWhere(d, keys, c.Key, param) }

	switch c.Kind {
	case "update":
		names, err := valueCols()
		if err != nil {
			return "", nil, err
		}
		if len(names) == 0 {
			return "", nil, errors.New("no hay columnas que modificar")
		}
		sets := make([]string, len(names))
		for i, n := range names {
			sets[i] = d.Quote(n) + " = " + param(c.Values[n])
		}
		w, err := where()
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("UPDATE %s SET %s WHERE %s", tbl, strings.Join(sets, ", "), w), args, nil
	case "insert":
		names, err := valueCols()
		if err != nil {
			return "", nil, err
		}
		if len(names) == 0 {
			if d.Name() == "mysql" {
				return "INSERT INTO " + tbl + " () VALUES ()", nil, nil
			}
			return "INSERT INTO " + tbl + " DEFAULT VALUES", nil, nil
		}
		cols := make([]string, len(names))
		ph := make([]string, len(names))
		for i, n := range names {
			cols[i] = d.Quote(n)
			ph[i] = param(c.Values[n])
		}
		return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tbl, strings.Join(cols, ", "), strings.Join(ph, ", ")), args, nil
	case "delete":
		w, err := where()
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("DELETE FROM %s WHERE %s", tbl, w), args, nil
	}
	return "", nil, fmt.Errorf("tipo de cambio desconocido: %q", c.Kind)
}
