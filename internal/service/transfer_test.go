package service

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newExport(t *testing.T) (*ExportService, *sql.DB, string) {
	t.Helper()
	_, db := newData(t)
	return NewExportService(fakePools{db, mustDialect(t)}, &Events{}), db, t.TempDir()
}

func tableReq(dir, format string) ExportRequest {
	return ExportRequest{Profile: "p", Kind: "table", Table: "items", Format: format, Path: filepath.Join(dir, "out."+format), Header: true}
}

func TestExportTableFormats(t *testing.T) {
	s, db, dir := newExport(t)
	db.Exec(`UPDATE items SET nota = 'tiene, "comillas"' WHERE id = 1`)

	// CSV
	res, err := s.Run("e1", tableReq(dir, "csv"))
	if err != nil || res.Rows != 25 || res.Bytes == 0 || res.Cancelled {
		t.Fatalf("csv: %v %+v", err, res)
	}
	f, _ := os.Open(tableReq(dir, "csv").Path)
	rec, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(rec) != 26 || rec[0][1] != "nombre" || rec[1][2] != `tiene, "comillas"` || rec[2][2] != "" {
		t.Fatalf("contenido CSV: %v %v", err, rec[:3])
	}

	// JSON
	if _, err := s.Run("e2", tableReq(dir, "json")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(tableReq(dir, "json").Path)
	var objs []map[string]any
	if err := json.Unmarshal(data, &objs); err != nil || len(objs) != 25 || objs[0]["nombre"] != "item01" || objs[1]["nota"] != nil {
		t.Fatalf("JSON: %v %v", err, objs[:2])
	}

	// SQL
	if res, err := s.Run("e3", tableReq(dir, "sql")); err != nil || res.Rows != 25 {
		t.Fatalf("sql: %v %+v", err, res)
	}
	sqlText, _ := os.ReadFile(tableReq(dir, "sql").Path)
	if !strings.Contains(string(sqlText), `INSERT INTO "items" ("id", "nombre", "nota", "qty") VALUES`) || strings.Count(string(sqlText), "INSERT INTO") != 1 {
		t.Errorf("INSERT por lotes de 100: %.200s", sqlText)
	}
}

func TestExportHasNoRowLimit(t *testing.T) {
	s, db, dir := newExport(t)
	tx, _ := db.Begin()
	for i := 100; i < 3100; i++ {
		tx.Exec(`INSERT INTO items (id, nombre) VALUES (?, ?)`, i, fmt.Sprintf("bulk%d", i))
	}
	tx.Commit()
	res, err := s.Run("big", tableReq(dir, "csv"))
	if err != nil || res.Rows != 3025 {
		t.Fatalf("debe exportar todas las filas, no solo las 1000 de la vista: %v %+v", err, res)
	}
}

func TestExportHonorsFilterAndOrder(t *testing.T) {
	s, _, dir := newExport(t)
	req := tableReq(dir, "csv")
	req.Filter, req.OrderBy, req.Desc = "id > 20", "id", true
	res, err := s.Run("f", req)
	if err != nil || res.Rows != 5 {
		t.Fatalf("filtro: %v %+v", err, res)
	}
	f, _ := os.Open(req.Path)
	defer f.Close()
	rec, _ := csv.NewReader(f).ReadAll()
	if rec[1][0] != "25" || rec[5][0] != "21" {
		t.Errorf("orden descendente: %v", rec)
	}
	req.Filter = "1=1; DROP TABLE items"
	if _, err := s.Run("f2", req); err == nil {
		t.Error("un filtro con ';' debe rechazarse")
	}
}

func TestExportQuery(t *testing.T) {
	s, _, dir := newExport(t)
	req := ExportRequest{Profile: "p", Kind: "query", SQL: "SELECT id, nombre FROM items WHERE id <= 3 ORDER BY id;", Format: "sql", SQLTable: "copia", Path: filepath.Join(dir, "q.sql")}
	res, err := s.Run("q", req)
	if err != nil || res.Rows != 3 {
		t.Fatalf("%v %+v", err, res)
	}
	got, _ := os.ReadFile(req.Path)
	if !strings.Contains(string(got), `INSERT INTO "copia" ("id", "nombre") VALUES`) || !strings.Contains(string(got), "(3, 'item03')") {
		t.Errorf("INSERT de una consulta: %s", got)
	}
	req.SQLTable = "esq.tabla"
	s.Run("q2", req)
	if got, _ := os.ReadFile(req.Path); !strings.Contains(string(got), `INSERT INTO "esq"."tabla"`) {
		t.Errorf("nombre cualificado: %s", got)
	}
	for _, bad := range []string{"DELETE FROM items", "DROP TABLE items", "INSERT INTO items(id,nombre) VALUES (999,'x')"} {
		r := req
		r.SQL = bad
		if _, err := s.Run("bad", r); err == nil || !strings.Contains(err.Error(), "devuelven filas") {
			t.Errorf("%q: solo se exportan consultas que devuelven filas: %v", bad, err)
		}
	}
}

// El SQL exportado debe poder reimportarse y dejar los mismos datos, incluidos los valores difíciles.
func TestExportSQLRoundtrip(t *testing.T) {
	s, db, dir := newExport(t)
	db.Exec(`CREATE TABLE raro (id INTEGER PRIMARY KEY, txt TEXT, num REAL, bin BLOB, n INT)`)
	tricky := []any{"O'Brien", `a\b`, "línea1\nlínea2", "ñandú 日本語", "", "--; DROP TABLE x"}
	for i, v := range tricky {
		db.Exec(`INSERT INTO raro VALUES (?, ?, ?, ?, ?)`, i+1, v, 1.5*float64(i), []byte{0xff, byte(i)}, nil)
	}
	req := ExportRequest{Profile: "p", Kind: "table", Table: "raro", Format: "sql", Path: filepath.Join(dir, "raro.sql")}
	if _, err := s.Run("rt", req); err != nil {
		t.Fatal(err)
	}
	script, _ := os.ReadFile(req.Path)

	// Recrear en una base nueva y comparar fila a fila.
	db2, _ := sql.Open("sqlite", filepath.Join(t.TempDir(), "copia.db"))
	defer db2.Close()
	db2.Exec(`CREATE TABLE raro (id INTEGER PRIMARY KEY, txt TEXT, num REAL, bin BLOB, n INT)`)
	if _, err := db2.Exec(string(script)); err != nil {
		t.Fatalf("el SQL exportado no se puede ejecutar: %v\n%s", err, script)
	}
	dump := func(d *sql.DB) string {
		rows, _ := d.Query(`SELECT id, txt, num, hex(bin), n FROM raro ORDER BY id`)
		defer rows.Close()
		var b strings.Builder
		for rows.Next() {
			var id int
			var txt, h sql.NullString
			var num float64
			var n sql.NullInt64
			rows.Scan(&id, &txt, &num, &h, &n)
			fmt.Fprintf(&b, "%d|%q|%v|%s|%v\n", id, txt.String, num, h.String, n.Valid)
		}
		return b.String()
	}
	if a, b := dump(db), dump(db2); a != b {
		t.Errorf("los datos difieren tras el viaje de ida y vuelta:\n--- original\n%s--- copia\n%s", a, b)
	}
}

func TestExportCancelLeavesNoFile(t *testing.T) {
	s, _, dir := newExport(t)
	req := ExportRequest{Profile: "p", Kind: "query", Format: "csv", Path: filepath.Join(dir, "grande.csv"),
		SQL: "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x < 50000000) SELECT x, x*2 FROM c"}
	done := make(chan *ExportResult, 1)
	go func() { r, _ := s.Run("cancelme", req); done <- r }()
	time.Sleep(150 * time.Millisecond)
	s.Cancel("cancelme")
	select {
	case r := <-done:
		if r == nil || !r.Cancelled {
			t.Fatalf("debía terminar como cancelada: %+v", r)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("la cancelación no cortó la exportación")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("cancelar no debe dejar archivos (ni .part): %v", ents)
	}
}

func TestExportFailureDoesNotClobberExistingFile(t *testing.T) {
	s, _, dir := newExport(t)
	dest := filepath.Join(dir, "importante.csv")
	os.WriteFile(dest, []byte("ORIGINAL"), 0o644)
	req := ExportRequest{Profile: "p", Kind: "query", SQL: "SELECT * FROM no_existe", Format: "csv", Path: dest}
	if _, err := s.Run("x", req); err == nil {
		t.Fatal("debía fallar")
	}
	if got, _ := os.ReadFile(dest); string(got) != "ORIGINAL" {
		t.Errorf("un fallo no debe pisar el archivo existente: %q", got)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("quedó un .part: %v", ents)
	}
	// Un éxito sí lo reemplaza.
	req.SQL = "SELECT 1 AS a"
	if _, err := s.Run("y", req); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) == "ORIGINAL" {
		t.Error("un éxito debe reemplazar el archivo")
	}
}

func TestExportRejectsBadRequests(t *testing.T) {
	s, _, dir := newExport(t)
	ok := tableReq(dir, "csv")
	cases := map[string]func(r *ExportRequest){
		"sin destino":         func(r *ExportRequest) { r.Path = "" },
		"formato desconocido": func(r *ExportRequest) { r.Format = "xlsx" },
		"carpeta inexistente": func(r *ExportRequest) { r.Path = filepath.Join(dir, "no", "existe", "a.csv") },
		"tipo desconocido":    func(r *ExportRequest) { r.Kind = "todo" },
		"tabla inexistente":   func(r *ExportRequest) { r.Table = "no_existe" },
		"orden inexistente":   func(r *ExportRequest) { r.OrderBy = "zzz" },
	}
	for name, mutate := range cases {
		r := ok
		mutate(&r)
		if _, err := s.Run("bad", r); err == nil {
			t.Errorf("%s: debería fallar", name)
		}
	}
}
