package service

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/SrChr1s/pluton-sql/internal/driver"
)

type fakePools struct {
	db *sql.DB
	d  driver.Dialect
}

func (f fakePools) Get(context.Context, string, string) (*sql.DB, driver.Dialect, error) {
	return f.db, f.d, nil
}

func newData(t *testing.T) (*DataService, *sql.DB) {
	t.Helper()
	d, err := driver.Get("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, nombre TEXT NOT NULL, nota TEXT, qty INT DEFAULT 5)`,
		`CREATE TABLE sinpk (a TEXT, b INT)`,
		`CREATE VIEW v_items AS SELECT id, nombre FROM items`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 25; i++ {
		db.Exec(`INSERT INTO items (id, nombre, nota) VALUES (?, ?, ?)`, i, fmt.Sprintf("item%02d", i), nil)
	}
	return NewDataService(fakePools{db, d}), db
}

func mustDialect(t *testing.T) driver.Dialect {
	t.Helper()
	d, err := driver.Get("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func ref(table string) TableRef { return TableRef{Profile: "p", Table: table} }

func TestPage(t *testing.T) {
	s, _ := newData(t)
	p, err := s.Page(PageRequest{Table: ref("items"), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 10 || !p.HasMore || !p.HasPK || len(p.KeyColumns) != 1 || p.KeyColumns[0] != "id" || p.IsView {
		t.Fatalf("página 1: filas=%d hasMore=%v pk=%v keys=%v", len(p.Rows), p.HasMore, p.HasPK, p.KeyColumns)
	}
	if p.Rows[0][0] != int64(1) || p.Rows[9][0] != int64(10) {
		t.Errorf("orden por clave primaria roto: %v ... %v", p.Rows[0], p.Rows[9])
	}
	if !p.Columns[0].PrimaryKey || p.Columns[1].Nullable {
		t.Errorf("metadatos de columnas: %+v", p.Columns)
	}

	last, _ := s.Page(PageRequest{Table: ref("items"), Limit: 10, Offset: 20})
	if len(last.Rows) != 5 || last.HasMore {
		t.Errorf("última página: filas=%d hasMore=%v", len(last.Rows), last.HasMore)
	}

	desc, _ := s.Page(PageRequest{Table: ref("items"), Limit: 3, OrderBy: "id", Desc: true})
	if desc.Rows[0][0] != int64(25) {
		t.Errorf("orden descendente: %v", desc.Rows[0])
	}
	f, err := s.Page(PageRequest{Table: ref("items"), Filter: "id > 22 AND nombre LIKE 'item%';"})
	if err != nil || len(f.Rows) != 3 {
		t.Errorf("filtro: %v filas=%d", err, len(f.Rows))
	}
	n, err := s.Count(ref("items"), "id <= 7")
	if err != nil || n != 7 {
		t.Errorf("Count = %d, %v", n, err)
	}

	if _, err := s.Page(PageRequest{Table: ref("items"), OrderBy: "id; DROP TABLE items"}); err == nil {
		t.Error("una columna de orden inexistente debe rechazarse")
	}
	if _, err := s.Page(PageRequest{Table: ref("items"), Filter: "1=1; DELETE FROM items"}); err == nil {
		t.Error("un filtro con ';' debe rechazarse")
	}
	if _, err := s.Page(PageRequest{Table: ref("no_existe")}); err == nil {
		t.Error("tabla inexistente debe dar error")
	}
	v, err := s.Page(PageRequest{Table: ref("v_items")})
	if err != nil || !v.IsView {
		t.Errorf("vista: %v isView=%v", err, v != nil && v.IsView)
	}
}

func count(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestApplyCRUD(t *testing.T) {
	s, db := newData(t)
	res, err := s.Apply(ref("items"), []Change{
		// JSON entrega los enteros como float64: debe aceptarse para una columna entera.
		{Kind: "update", Key: map[string]any{"id": float64(1), "nombre": "item01"}, Values: map[string]any{"nombre": "renombrado", "nota": nil, "qty": "42"}},
		{Kind: "insert", Values: map[string]any{"nombre": "nuevo"}},
		{Kind: "delete", Key: map[string]any{"id": float64(2)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || res.Inserted != 1 || res.Deleted != 1 {
		t.Errorf("resultado = %+v", res)
	}
	var nombre string
	var nota sql.NullString
	var qty int
	db.QueryRow(`SELECT nombre, nota, qty FROM items WHERE id = 1`).Scan(&nombre, &nota, &qty)
	if nombre != "renombrado" || nota.Valid || qty != 42 {
		t.Errorf("update: %q %v %d", nombre, nota, qty)
	}
	if count(t, db, `SELECT count(*) FROM items WHERE nombre = 'nuevo' AND qty = 5`) != 1 {
		t.Error("insert con valor por defecto")
	}
	if count(t, db, `SELECT count(*) FROM items WHERE id = 2`) != 0 {
		t.Error("delete")
	}
	if _, err := s.Apply(ref("items"), []Change{{Kind: "insert", Values: map[string]any{}}}); err == nil {
		// nombre es NOT NULL: sin valores debe fallar, no insertar basura
		t.Error("insert vacío en tabla con NOT NULL debería fallar")
	}
}

func TestApplyIsAtomic(t *testing.T) {
	s, db := newData(t)
	_, err := s.Apply(ref("items"), []Change{
		{Kind: "update", Key: map[string]any{"id": 1}, Values: map[string]any{"nombre": "cambiado"}},
		{Kind: "update", Key: map[string]any{"id": 9999}, Values: map[string]any{"nombre": "x"}}, // no existe
	})
	if err == nil || !strings.Contains(err.Error(), "cambio 2") {
		t.Fatalf("debía fallar en el cambio 2: %v", err)
	}
	if count(t, db, `SELECT count(*) FROM items WHERE nombre = 'cambiado'`) != 0 {
		t.Error("el primer cambio no se revirtió")
	}
}

func TestApplyWithoutPrimaryKey(t *testing.T) {
	s, db := newData(t)
	db.Exec(`INSERT INTO sinpk VALUES ('unica', 1), ('dup', 2), ('dup', 2), (NULL, 3)`)

	p, _ := s.Page(PageRequest{Table: ref("sinpk")})
	if p.HasPK || len(p.KeyColumns) != 2 {
		t.Fatalf("sin PK debe usar todas las columnas: pk=%v keys=%v", p.HasPK, p.KeyColumns)
	}

	// Fila única: se puede editar.
	if _, err := s.Apply(ref("sinpk"), []Change{
		{Kind: "update", Key: map[string]any{"a": "unica", "b": 1}, Values: map[string]any{"b": 10}},
	}); err != nil {
		t.Fatal(err)
	}
	// Clave con NULL: debe comparar con IS NULL y no con = NULL.
	if _, err := s.Apply(ref("sinpk"), []Change{
		{Kind: "delete", Key: map[string]any{"a": nil, "b": 3}},
	}); err != nil {
		t.Fatalf("clave con NULL: %v", err)
	}
	// Filas duplicadas: no se distinguen, así que se rechaza y no se toca ninguna.
	_, err := s.Apply(ref("sinpk"), []Change{
		{Kind: "delete", Key: map[string]any{"a": "dup", "b": 2}},
	})
	if err == nil || !strings.Contains(err.Error(), "afectaron 2") {
		t.Fatalf("filas duplicadas deben rechazarse: %v", err)
	}
	if count(t, db, `SELECT count(*) FROM sinpk WHERE a = 'dup'`) != 2 {
		t.Error("se borró alguna fila duplicada pese al error")
	}
}

func TestApplyRejectsBadInput(t *testing.T) {
	s, db := newData(t)
	cases := map[string]Change{
		"columna inexistente":     {Kind: "update", Key: map[string]any{"id": 1}, Values: map[string]any{"nombre": "x", "zzz": 1}},
		"inyección en la columna": {Kind: "insert", Values: map[string]any{"nombre) VALUES ('a'); DROP TABLE items; --": 1}},
		"update sin columnas":     {Kind: "update", Key: map[string]any{"id": 1}},
		"falta la clave":          {Kind: "delete", Key: map[string]any{}},
		"tipo desconocido":        {Kind: "truncate"},
	}
	for name, c := range cases {
		if _, err := s.Apply(ref("items"), []Change{c}); err == nil {
			t.Errorf("%s: debería fallar", name)
		}
	}
	if count(t, db, `SELECT count(*) FROM items`) != 25 {
		t.Error("alguna entrada inválida modificó la tabla")
	}
	if _, err := s.Apply(ref("v_items"), []Change{{Kind: "delete", Key: map[string]any{"id": 1}}}); err == nil {
		t.Error("las vistas no se pueden modificar")
	}
}

func TestCoerce(t *testing.T) {
	if coerce(float64(5)) != int64(5) || coerce(1.5) != 1.5 || coerce("5") != "5" || coerce(nil) != nil {
		t.Error("coerce")
	}
	if _, ok := coerce(float64(1 << 60)).(float64); !ok {
		t.Error("un float enorme no debe convertirse a int64")
	}
}
