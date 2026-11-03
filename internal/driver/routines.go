package driver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// RoutineInfo es un procedimiento almacenado o una función.
type RoutineInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
	Type   string `json:"type"` // procedure | function
	// Args es la firma, solo para distinguir sobrecargas (PostgreSQL); "" si el motor no la da.
	Args string `json:"args"`
	// ID identifica la rutina al pedir su definición (el oid en PostgreSQL).
	ID string `json:"id"`
}

var errNoRoutines = errors.New("este motor no tiene procedimientos almacenados")

func scanRoutines(rows *sql.Rows, schema string) ([]RoutineInfo, error) {
	defer rows.Close()
	var out []RoutineInfo
	for rows.Next() {
		r := RoutineInfo{Schema: schema}
		var kind string
		if err := rows.Scan(&r.Name, &kind, &r.Args, &r.ID); err != nil {
			return nil, err
		}
		r.Type = "function"
		if strings.HasPrefix(strings.ToUpper(kind), "P") {
			r.Type = "procedure"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- PostgreSQL ---

// Se excluyen las rutinas que instala una extensión (deptype 'e'): serían cientos de ruido.
const pgRoutines = `SELECT p.proname, %s, pg_get_function_identity_arguments(p.oid), p.oid::text
	FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
	WHERE n.nspname = $1 AND %s
	AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = p.oid AND d.deptype = 'e')
	ORDER BY p.proname, p.oid`

func (postgres) ListRoutines(ctx context.Context, db *sql.DB, schema string) ([]RoutineInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(pgRoutines, `p.prokind::text`, `p.prokind IN ('f','p')`), schema)
	if err != nil { // PostgreSQL < 11: no hay prokind ni procedimientos, solo funciones
		rows, err = db.QueryContext(ctx, fmt.Sprintf(pgRoutines, `'f'`, `NOT p.proisagg`), schema)
		if err != nil {
			return nil, err
		}
	}
	return scanRoutines(rows, schema)
}

func (postgres) RoutineDDL(ctx context.Context, db *sql.DB, _ string, r RoutineInfo) (string, error) {
	var def string
	if err := db.QueryRowContext(ctx, `SELECT pg_get_functiondef($1::oid)`, r.ID).Scan(&def); err != nil {
		return "", err
	}
	return strings.TrimSpace(def), nil
}

// --- MySQL / MariaDB ---

func (mysqlDialect) ListRoutines(ctx context.Context, db *sql.DB, _ string) ([]RoutineInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT routine_name, routine_type, '', routine_name
		FROM information_schema.routines WHERE routine_schema = DATABASE() ORDER BY routine_name`)
	if err != nil {
		return nil, err
	}
	return scanRoutines(rows, "")
}

func (d mysqlDialect) RoutineDDL(ctx context.Context, db *sql.DB, _ string, r RoutineInfo) (string, error) {
	kind := "FUNCTION"
	if r.Type == "procedure" {
		kind = "PROCEDURE"
	}
	rows, err := db.QueryContext(ctx, "SHOW CREATE "+kind+" "+d.Quote(r.Name))
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
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	for i, c := range cols {
		if strings.HasPrefix(c, "Create ") { // "Create Procedure" / "Create Function"
			if !vals[i].Valid {
				return "", errors.New("sin permiso para ver la definición")
			}
			return strings.TrimSpace(vals[i].String), nil
		}
	}
	return "", errors.New("el servidor no devolvió la definición")
}

// --- SQL Server ---

func (mssql) ListRoutines(ctx context.Context, db *sql.DB, schema string) ([]RoutineInfo, error) {
	rows, err := db.QueryContext(ctx, `SELECT o.name, o.type, '', o.name
		FROM sys.objects o JOIN sys.schemas s ON s.schema_id = o.schema_id
		WHERE s.name = @p1 AND o.type IN ('P','FN','IF','TF') AND o.is_ms_shipped = 0
		ORDER BY o.name`, schema)
	if err != nil {
		return nil, err
	}
	return scanRoutines(rows, schema)
}

func (m mssql) RoutineDDL(ctx context.Context, db *sql.DB, schema string, r RoutineInfo) (string, error) {
	var def sql.NullString
	qn := m.Quote(schema) + "." + m.Quote(r.Name)
	if err := db.QueryRowContext(ctx, `SELECT OBJECT_DEFINITION(OBJECT_ID(@p1))`, qn).Scan(&def); err != nil {
		return "", err
	}
	if !def.Valid {
		return "", errors.New("no se pudo leer la definición (cifrada con WITH ENCRYPTION o sin permiso)")
	}
	return strings.TrimSpace(def.String), nil
}

// --- SQLite ---

func (sqlite) ListRoutines(context.Context, *sql.DB, string) ([]RoutineInfo, error) { return nil, nil }

func (sqlite) RoutineDDL(context.Context, *sql.DB, string, RoutineInfo) (string, error) {
	return "", errNoRoutines
}
