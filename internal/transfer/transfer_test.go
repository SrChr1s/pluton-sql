package transfer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type quoter string

func (q quoter) Name() string { return string(q) }
func (q quoter) Quote(i string) string {
	switch q {
	case "mysql":
		return "`" + i + "`"
	case "mssql":
		return "[" + i + "]"
	}
	return `"` + i + `"`
}

func write(t *testing.T, w Writer, cols []string, rows ...[]any) {
	t.Helper()
	if err := w.Begin(cols); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := w.Row(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.End(); err != nil {
		t.Fatal(err)
	}
}

var ts = time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC)

func TestCSV(t *testing.T) {
	var b bytes.Buffer
	write(t, NewCSV(&b, CSVOptions{Header: true}), []string{"id", "nombre", "nota", "alta", "x"},
		[]any{int64(1), "Ana, \"la\" grande", "línea1\nlínea2", ts, nil},
		[]any{int64(9007199254740993), []byte("texto"), []byte{0xff, 0x00}, 1.5, true})
	// Releer con un lector CSV estándar: lo que se escribe debe poder leerse sin pérdida.
	rec, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatalf("el CSV generado no es válido: %v", err)
	}
	if len(rec) != 3 || rec[0][0] != "id" {
		t.Fatalf("registros: %v", rec)
	}
	if rec[1][1] != "Ana, \"la\" grande" || rec[1][2] != "línea1\nlínea2" {
		t.Errorf("comas, comillas y saltos de línea deben sobrevivir: %q", rec[1])
	}
	if rec[1][3] != "2026-01-02 03:04:05.123456" || rec[1][4] != "" {
		t.Errorf("fecha y NULL: %q", rec[1])
	}
	if rec[2][0] != "9007199254740993" || rec[2][1] != "texto" || rec[2][2] != "0xff00" || rec[2][3] != "1.5" || rec[2][4] != "true" {
		t.Errorf("enteros grandes exactos, bytes y tipos: %q", rec[2])
	}
}

func TestCSVTimeKeepsOffset(t *testing.T) {
	var b bytes.Buffer
	cet := time.FixedZone("", -3*3600)
	write(t, NewCSV(&b, CSVOptions{}), []string{"t"}, []any{time.Date(2026, 1, 2, 3, 4, 5, 0, cet)})
	if got := strings.TrimSpace(b.String()); got != "2026-01-02 03:04:05-03:00" {
		t.Errorf("un valor con desfase debe conservarlo (si no, cambia el instante): %q", got)
	}
}

func TestCSVOptions(t *testing.T) {
	var b bytes.Buffer
	write(t, NewCSV(&b, CSVOptions{Delimiter: ';', Header: false, BOM: true}), []string{"a", "b"}, []any{"x;y", int64(2)})
	if !strings.HasPrefix(b.String(), "\xef\xbb\xbf") {
		t.Error("falta el BOM")
	}
	if got := strings.TrimPrefix(b.String(), "\xef\xbb\xbf"); got != "\"x;y\";2\n" {
		t.Errorf("separador ; y sin cabecera: %q", got)
	}
	b.Reset()
	write(t, NewCSV(&b, CSVOptions{Header: true}), []string{"id", "id", "id"})
	if b.String() != "id,id_2,id_3\n" {
		t.Errorf("columnas repetidas: %q", b.String())
	}
}

func TestJSON(t *testing.T) {
	var b bytes.Buffer
	write(t, NewJSON(&b), []string{"id", "n", "s", "f", "nan", "t", "bin", "id"},
		[]any{int64(9007199254740993), nil, "a\"b\n", 1.5, math.NaN(), ts, []byte{0xff}, int64(2)},
		[]any{int64(2), "x", "ñ", math.Inf(1), 0.0, "t", []byte("ok"), int64(3)})
	var got []map[string]any
	dec := json.NewDecoder(bytes.NewReader(b.Bytes()))
	dec.UseNumber() // para comprobar que el entero grande no pasó por float64
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, b.String())
	}
	if len(got) != 2 {
		t.Fatalf("filas = %d", len(got))
	}
	if got[0]["id"].(json.Number).String() != "9007199254740993" {
		t.Errorf("el entero grande debe ser exacto: %v", got[0]["id"])
	}
	if got[0]["n"] != nil || got[0]["s"] != "a\"b\n" || got[0]["nan"] != nil || got[0]["t"] != "2026-01-02T03:04:05.123456Z" || got[0]["bin"] != "0xff" {
		t.Errorf("fila 1: %v", got[0])
	}
	if got[0]["id_2"].(json.Number).String() != "2" || got[1]["f"] != nil || got[1]["bin"] != "ok" || got[1]["s"] != "ñ" {
		t.Errorf("columnas repetidas y valores: %v %v", got[0], got[1])
	}
	b.Reset()
	write(t, NewJSON(&b), []string{"a"})
	var empty []any
	if err := json.Unmarshal(b.Bytes(), &empty); err != nil || len(empty) != 0 {
		t.Errorf("sin filas debe ser un array vacío válido: %q %v", b.String(), err)
	}
}

func TestSQLLiterals(t *testing.T) {
	cases := []struct {
		engine string
		v      any
		want   string
	}{
		{"postgres", nil, "NULL"},
		{"postgres", "O'Brien", "'O''Brien'"},
		{"postgres", `a\b`, `'a\b'`}, // en PostgreSQL la barra no escapa
		{"mysql", `a\b`, `'a\\b'`},   // en MySQL sí
		{"mysql", "O'Brien", "'O''Brien'"},
		{"mssql", "ñandú", "N'ñandú'"},
		{"sqlite", "x", "'x'"},
		{"postgres", true, "TRUE"},
		{"mysql", false, "FALSE"},
		{"mssql", true, "1"},
		{"sqlite", false, "0"},
		{"postgres", int64(-5), "-5"},
		{"postgres", 1.5, "1.5"},
		{"postgres", math.NaN(), "NULL"},
		{"postgres", ts, "'2026-01-02 03:04:05.123456Z'"},
		{"mysql", ts, "'2026-01-02 03:04:05.123456'"},
		{"postgres", []byte{0xff, 0xfe}, `'\xfffe'`},
		{"mysql", []byte{0xff, 0xfe}, "X'fffe'"},
		{"sqlite", []byte{0xff, 0xfe}, "X'fffe'"},
		{"mssql", []byte{0xff, 0xfe}, "0xfffe"},
		{"postgres", []byte("texto"), "'texto'"},
	}
	for _, c := range cases {
		if got := Literal(c.engine, c.v); got != c.want {
			t.Errorf("%s %#v → %s, quería %s", c.engine, c.v, got, c.want)
		}
	}
}

func TestSQLWriterBatches(t *testing.T) {
	var b bytes.Buffer
	w := NewSQL(&b, quoter("postgres"), `"public"."t"`, 2)
	write(t, w, []string{"id", "nombre"}, []any{int64(1), "a"}, []any{int64(2), "b"}, []any{int64(3), "c"})
	want := "INSERT INTO \"public\".\"t\" (\"id\", \"nombre\") VALUES\n  (1, 'a'),\n  (2, 'b');\nINSERT INTO \"public\".\"t\" (\"id\", \"nombre\") VALUES\n  (3, 'c');\n"
	if b.String() != want {
		t.Errorf("lotes de 2:\n%s\nquería:\n%s", b.String(), want)
	}
	b.Reset()
	write(t, NewSQL(&b, quoter("mysql"), "`t`", 100), []string{"a"})
	if b.String() != "" {
		t.Errorf("sin filas no debe escribir ninguna sentencia: %q", b.String())
	}
	b.Reset()
	write(t, NewSQL(&b, quoter("mysql"), "`t`", 100), []string{"x", "x"}, []any{int64(1), int64(2)})
	if !strings.Contains(b.String(), "(`x`, `x_2`)") {
		t.Errorf("columnas repetidas en INSERT: %s", b.String())
	}
}

func TestDetectDelimiter(t *testing.T) {
	cases := map[string]rune{
		"a,b,c\n1,2,3\n":                      ',',
		"a;b;c\n1;2;3\n":                      ';',
		"a\tb\tc\n1\t2\t3\n":                  '\t',
		"a|b\n1|2\n":                          '|',
		"nombre;precio\nlápiz;1,50\ngoma;2,5": ';', // la coma decimal no debe engañar
		"id;nombre\n1;\"a; b\"\n2;c\n":        ';', // el separador dentro de comillas no cuenta
		"a,b\n\"x,y\",z\n":                    ',',
		"solo una columna\notra\n":            ',',
		"":                                    ',',
	}
	for in, want := range cases {
		if got := DetectDelimiter(in); got != want {
			t.Errorf("%q → %q, quería %q", in, got, want)
		}
	}
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadPreview(t *testing.T) {
	p, err := ReadPreview(writeFile(t, "a.csv", []byte("\xef\xbb\xbfid;nombre\n1;Ana\n2;\"Luis; el \"\"grande\"\"\"\n3;Eva\n")), 0, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Delimiter != ";" || len(p.Header) != 2 || p.Header[0] != "id" {
		t.Errorf("BOM, separador y cabecera: %+v", p)
	}
	if len(p.Rows) != 2 || p.Rows[1][1] != "Luis; el \"grande\"" {
		t.Errorf("filas (límite 2, con comillas): %+v", p.Rows)
	}

	// Sin cabecera: se generan nombres.
	p, _ = ReadPreview(writeFile(t, "b.csv", []byte("1,Ana,x\n2,Luis,y\n")), 0, false, 10)
	if len(p.Rows) != 2 || p.Columns != 3 || p.Header[0] != "columna1" || p.Header[2] != "columna3" {
		t.Errorf("sin cabecera: %+v", p)
	}
}

func TestReadPreviewProblems(t *testing.T) {
	if p, err := ReadPreview(writeFile(t, "r.csv", []byte("a,b\n1,2\n3\n4,5,6\n")), ',', true, 10); err != nil || len(p.Warnings) == 0 || p.Columns != 3 {
		t.Errorf("filas irregulares deben avisar: %v %+v", err, p)
	}
	if p, _ := ReadPreview(writeFile(t, "h.csv", []byte("a,b\n")), ',', true, 10); len(p.Warnings) == 0 {
		t.Errorf("solo cabecera debe avisar: %+v", p)
	}
	if _, err := ReadPreview(writeFile(t, "e.csv", nil), ',', true, 10); err == nil {
		t.Error("un archivo vacío debe dar error")
	}
	if _, err := ReadPreview(writeFile(t, "l.csv", []byte("a,b\n1,\xf1\n")), ',', true, 10); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("Latin-1 debe rechazarse con un mensaje claro: %v", err)
	}
	if _, err := ReadPreview(writeFile(t, "q.csv", []byte("a,b\n1,\"sin cerrar\n")), ',', true, 10); err == nil {
		t.Error("comillas sin cerrar deben dar error")
	}
	if _, err := ReadPreview(filepath.Join(t.TempDir(), "no.csv"), ',', true, 10); err == nil {
		t.Error("archivo inexistente")
	}
}
