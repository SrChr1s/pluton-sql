package service

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newImport(t *testing.T) (*ImportService, *sql.DB) {
	t.Helper()
	_, db := newData(t)
	for _, q := range []string{
		`CREATE TABLE destino (id INTEGER PRIMARY KEY, nombre TEXT NOT NULL, edad INT, nota TEXT, fecha TEXT DEFAULT 'hoy')`,
		`CREATE VIEW v_destino AS SELECT id FROM destino`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return NewImportService(fakePools{db, mustDialect(t)}, &Events{}), db
}

func csvFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "datos.csv")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func importReq(path string, mapping ...ImportMap) ImportRequest {
	return ImportRequest{Table: ref("destino"), Path: path, HasHeader: true, Mapping: mapping}
}

func rowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	return count(t, db, "SELECT count(*) FROM "+table)
}

func TestImportHappyPath(t *testing.T) {
	s, db := newImport(t)
	p := csvFile(t, "\xef\xbb\xbfnombre;años;comentario\nAna;30;\"hola; mundo\"\nLuis;;\nEva;41;\n")
	prev, err := s.Preview(p, "", true)
	if err != nil || prev.Delimiter != ";" || prev.Header[1] != "años" || len(prev.Rows) != 3 {
		t.Fatalf("vista previa (BOM, separador detectado): %v %+v", err, prev)
	}
	res, err := s.Run("i", importReq(p, ImportMap{"nombre", 0}, ImportMap{"edad", 1}, ImportMap{"nota", 2}))
	if err != nil || res.Inserted != 3 {
		t.Fatalf("%v %+v", err, res)
	}
	type row struct {
		nombre string
		edad   sql.NullInt64
		nota   sql.NullString
		fecha  string
	}
	var got []row
	rows, _ := db.Query(`SELECT nombre, edad, nota, fecha FROM destino ORDER BY id`)
	for rows.Next() {
		var r row
		rows.Scan(&r.nombre, &r.edad, &r.nota, &r.fecha)
		got = append(got, r)
	}
	rows.Close()
	if got[0].nota.String != "hola; mundo" || got[0].edad.Int64 != 30 {
		t.Errorf("fila 1: %+v", got[0])
	}
	if got[1].edad.Valid {
		t.Errorf("una cadena vacía en una columna numérica debe ser NULL: %+v", got[1])
	}
	if !got[1].nota.Valid || got[1].nota.String != "" {
		t.Errorf("una cadena vacía en una columna de texto se conserva como '' por defecto: %+v", got[1].nota)
	}
	if got[0].fecha != "hoy" {
		t.Errorf("las columnas no mapeadas usan su valor por defecto: %q", got[0].fecha)
	}
}

func TestImportEmptyAsNullOption(t *testing.T) {
	s, db := newImport(t)
	req := importReq(csvFile(t, "nombre,nota\nAna,\n"), ImportMap{"nombre", 0}, ImportMap{"nota", 1})
	req.EmptyAsNull = true
	if _, err := s.Run("i", req); err != nil {
		t.Fatal(err)
	}
	var nota sql.NullString
	db.QueryRow(`SELECT nota FROM destino`).Scan(&nota)
	if nota.Valid {
		t.Errorf("con EmptyAsNull el texto vacío debe ser NULL: %+v", nota)
	}
}

func TestImportNoHeaderAndPartialMapping(t *testing.T) {
	s, db := newImport(t)
	req := importReq(csvFile(t, "x,Ana,99\ny,Luis,98\n"), ImportMap{"nombre", 1}, ImportMap{"edad", 2}, ImportMap{"nota", -1})
	req.HasHeader = false
	if res, err := s.Run("i", req); err != nil || res.Inserted != 2 {
		t.Fatalf("%v %+v", err, res)
	}
	var n string
	db.QueryRow(`SELECT nombre FROM destino ORDER BY id LIMIT 1`).Scan(&n)
	if n != "Ana" {
		t.Errorf("mapeo por posición: %q", n)
	}
}

func TestImportIsAllOrNothing(t *testing.T) {
	s, db := newImport(t)
	// El id de la fila 4 está repetido: las 2 filas válidas anteriores no deben quedar guardadas.
	p := csvFile(t, "id,nombre\n1,Ana\n2,Luis\n2,Repetido\n5,Eva\n")
	_, err := s.Run("i", importReq(p, ImportMap{"id", 0}, ImportMap{"nombre", 1}))
	if err == nil || !strings.Contains(err.Error(), "fila 4") || !strings.Contains(err.Error(), "no se importó nada") {
		t.Fatalf("debe indicar la fila que falla: %v", err)
	}
	if n := rowCount(t, db, "destino"); n != 0 {
		t.Errorf("la importación fallida dejó %d filas", n)
	}
	// Fila con menos columnas de las que necesita el mapeo.
	_, err = s.Run("i", importReq(csvFile(t, "a,b\nAna,1\nsolo\n"), ImportMap{"nombre", 0}, ImportMap{"edad", 1}))
	if err == nil || !strings.Contains(err.Error(), "fila 3") {
		t.Errorf("fila corta: %v", err)
	}
	if n := rowCount(t, db, "destino"); n != 0 {
		t.Errorf("tras una fila corta quedaron %d filas", n)
	}
	// Archivo corrupto a mitad (comillas sin cerrar) tampoco deja nada.
	if _, err := s.Run("i", importReq(csvFile(t, "nombre\nAna\n\"sin cerrar\n"), ImportMap{"nombre", 0})); err == nil {
		t.Error("CSV malformado debe dar error")
	}
	if n := rowCount(t, db, "destino"); n != 0 {
		t.Errorf("tras un CSV malformado quedaron %d filas", n)
	}
}

func TestImportRejectsBadMapping(t *testing.T) {
	s, _ := newImport(t)
	p := csvFile(t, "a,b\n1,2\n")
	cases := map[string]ImportRequest{
		"sin mapeo":           importReq(p),
		"todo ignorado":       importReq(p, ImportMap{"nombre", -1}),
		"columna inexistente": importReq(p, ImportMap{"zzz", 0}),
		"columna repetida":    importReq(p, ImportMap{"nombre", 0}, ImportMap{"nombre", 1}),
	}
	for name, r := range cases {
		if _, err := s.Run("bad", r); err == nil {
			t.Errorf("%s: debería fallar", name)
		}
	}
	v := importReq(p, ImportMap{"id", 0})
	v.Table = ref("v_destino")
	if _, err := s.Run("bad", v); err == nil || !strings.Contains(err.Error(), "vista") {
		t.Errorf("importar en una vista: %v", err)
	}
	if _, err := s.Run("bad", importReq(filepath.Join(t.TempDir(), "no.csv"), ImportMap{"nombre", 0})); err == nil {
		t.Error("archivo inexistente")
	}
}

func TestImportCancelRollsBack(t *testing.T) {
	s, db := newImport(t)
	var b strings.Builder
	b.WriteString("nombre,edad\n")
	for i := 0; i < 400000; i++ {
		fmt.Fprintf(&b, "n%d,%d\n", i, i%90)
	}
	p := csvFile(t, b.String())
	done := make(chan *ImportResult, 1)
	go func() { r, _ := s.Run("cancel", importReq(p, ImportMap{"nombre", 0}, ImportMap{"edad", 1})); done <- r }()
	time.Sleep(300 * time.Millisecond)
	s.Cancel("cancel")
	select {
	case r := <-done:
		if r == nil || !r.Cancelled {
			t.Fatalf("debía terminar cancelada: %+v", r)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("la cancelación no cortó la importación")
	}
	if n := rowCount(t, db, "destino"); n != 0 {
		t.Errorf("cancelar debe revertir todo: quedaron %d filas", n)
	}
}
