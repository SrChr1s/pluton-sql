package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SrChr1s/pluton-sql/internal/driver"
)

func openSQLite(t *testing.T, stmts ...string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return db
}

// "a_hija" va antes que "z_padre" por nombre: solo el orden por claves foráneas las pone bien.
var scriptFixture = []string{
	`CREATE TABLE a_hija (id INTEGER PRIMARY KEY, padre_id INT NOT NULL REFERENCES z_padre(id), nota TEXT)`,
	`CREATE TABLE z_padre (id INTEGER PRIMARY KEY, nombre TEXT NOT NULL)`,
	`CREATE INDEX ix_hija_padre ON a_hija (padre_id)`,
	`CREATE VIEW v_hijas AS SELECT h.id, p.nombre FROM a_hija h JOIN z_padre p ON p.id = h.padre_id`,
	`INSERT INTO z_padre VALUES (1, 'uno'), (2, 'dos ''con comilla''')`,
	`INSERT INTO a_hija VALUES (10, 1, 'x'), (11, 2, NULL), (12, 2, 'ñandú')`,
}

func genScript(t *testing.T, db *sql.DB, opt ScriptOptions) (string, *ScriptStats) {
	t.Helper()
	d := mustDialect(t)
	var sb strings.Builder
	st, err := generateScript(context.Background(), db, d, "t", opt, &sb, nil)
	if err != nil {
		t.Fatal(err)
	}
	return sb.String(), st
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("%s: %v", table, err)
	}
	return n
}

func TestScriptRoundTrip(t *testing.T) {
	src := openSQLite(t, scriptFixture...)
	script, st := genScript(t, src, ScriptOptions{Mode: "both", IncludeDrop: true, IncludeIndexes: true, IncludeFKs: true})
	if len(st.Errors) != 0 {
		t.Fatalf("errores: %v", st.Errors)
	}
	if st.Rows != 5 || st.Objects != 3 {
		t.Fatalf("filas=%d objetos=%d", st.Rows, st.Objects)
	}

	// Orden: la tabla padre se crea antes que la hija aunque la hija vaya antes por nombre.
	pos := func(s string) int {
		i := strings.Index(script, s)
		if i < 0 {
			t.Fatalf("falta %q en el script:\n%s", s, script)
		}
		return i
	}
	if pos("CREATE TABLE z_padre") > pos("CREATE TABLE a_hija") {
		t.Errorf("la tabla hija se crea antes que la padre:\n%s", script)
	}
	if pos("INSERT INTO \"z_padre\"") > pos("INSERT INTO \"a_hija\"") {
		t.Errorf("los datos de la hija van antes que los del padre")
	}
	if pos("INSERT INTO \"a_hija\"") > pos("CREATE INDEX ix_hija_padre") {
		t.Errorf("el índice debe crearse después de los datos")
	}
	if pos("CREATE INDEX") > pos("CREATE VIEW") {
		t.Errorf("la vista debe ir después de índices")
	}
	if pos("PRAGMA foreign_keys=OFF") > pos("DROP TABLE") {
		t.Errorf("el prólogo debe ir antes de todo")
	}

	// Ejecutado sobre una base vacía reproduce los datos; ejecutado de nuevo (con DROP) es idempotente.
	dst := openSQLite(t)
	for i := 0; i < 2; i++ {
		if _, err := dst.Exec(script); err != nil {
			t.Fatalf("ejecución %d: %v\n%s", i+1, err, script)
		}
		if countRows(t, dst, "z_padre") != 2 || countRows(t, dst, "a_hija") != 3 || countRows(t, dst, "v_hijas") != 3 {
			t.Fatalf("ejecución %d: filas distintas", i+1)
		}
	}
	var nombre, nota string
	if err := dst.QueryRow(`SELECT nombre FROM z_padre WHERE id = 2`).Scan(&nombre); err != nil || nombre != "dos 'con comilla'" {
		t.Errorf("comillas: %q %v", nombre, err)
	}
	if err := dst.QueryRow(`SELECT nota FROM a_hija WHERE id = 12`).Scan(&nota); err != nil || nota != "ñandú" {
		t.Errorf("unicode: %q %v", nota, err)
	}
	var idx int
	dst.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='ix_hija_padre'`).Scan(&idx)
	if idx != 1 {
		t.Errorf("falta el índice")
	}
}

func TestScriptModes(t *testing.T) {
	src := openSQLite(t, scriptFixture...)

	schema, st := genScript(t, src, ScriptOptions{Mode: "schema", IncludeIndexes: true})
	if strings.Contains(schema, "INSERT INTO") || st.Rows != 0 {
		t.Errorf("el modo esquema no debe llevar datos:\n%s", schema)
	}
	if !strings.Contains(schema, "CREATE TABLE") || !strings.Contains(schema, "CREATE VIEW") {
		t.Errorf("el modo esquema debe llevar tablas y vistas:\n%s", schema)
	}
	if strings.Contains(schema, "DROP ") {
		t.Errorf("sin IncludeDrop no debe haber DROP")
	}

	data, st := genScript(t, src, ScriptOptions{Mode: "data"})
	if strings.Contains(data, "CREATE ") || st.Rows != 5 || st.Objects != 2 {
		t.Errorf("el modo datos solo lleva INSERT (filas=%d objetos=%d):\n%s", st.Rows, st.Objects, data)
	}

	// Solo los objetos pedidos.
	one, st := genScript(t, src, ScriptOptions{Mode: "both", Objects: []ScriptObject{{Name: "z_padre", Kind: "table"}}})
	if strings.Contains(one, "a_hija") || st.Rows != 2 {
		t.Errorf("solo debía salir z_padre:\n%s", one)
	}

	// Sin IncludeIndexes no salen índices.
	noIdx, _ := genScript(t, src, ScriptOptions{Mode: "schema"})
	if strings.Contains(noIdx, "CREATE INDEX") {
		t.Errorf("sin IncludeIndexes no debe haber índices")
	}
}

func TestScriptBatchRows(t *testing.T) {
	src := openSQLite(t, `CREATE TABLE n (id INTEGER PRIMARY KEY)`)
	for i := 1; i <= 25; i++ {
		src.Exec(`INSERT INTO n VALUES (?)`, i)
	}
	script, st := genScript(t, src, ScriptOptions{Mode: "data", BatchRows: 10})
	if got := strings.Count(script, "INSERT INTO"); got != 3 || st.Rows != 25 {
		t.Errorf("INSERT=%d filas=%d, se esperaban 3 sentencias y 25 filas", got, st.Rows)
	}
}

func TestScriptCircularReferences(t *testing.T) {
	src := openSQLite(t,
		`CREATE TABLE a (id INTEGER PRIMARY KEY, b_id INT REFERENCES b(id))`,
		`CREATE TABLE b (id INTEGER PRIMARY KEY, a_id INT REFERENCES a(id))`,
		`INSERT INTO a VALUES (1, NULL)`, `INSERT INTO b VALUES (1, 1)`)
	script, st := genScript(t, src, ScriptOptions{Mode: "both"})
	if st.Objects != 2 || len(st.Errors) != 0 {
		t.Fatalf("objetos=%d errores=%v", st.Objects, st.Errors)
	}
	dst := openSQLite(t)
	if _, err := dst.Exec(script); err != nil {
		t.Fatalf("%v\n%s", err, script)
	}
	if countRows(t, dst, "a") != 1 || countRows(t, dst, "b") != 1 {
		t.Error("filas distintas")
	}
}

func TestScriptSelfReference(t *testing.T) {
	src := openSQLite(t, `CREATE TABLE arbol (id INTEGER PRIMARY KEY, padre INT REFERENCES arbol(id))`,
		`INSERT INTO arbol VALUES (1, NULL), (2, 1)`)
	script, _ := genScript(t, src, ScriptOptions{Mode: "both"})
	dst := openSQLite(t)
	if _, err := dst.Exec(script); err != nil {
		t.Fatalf("%v\n%s", err, script)
	}
	if countRows(t, dst, "arbol") != 2 {
		t.Error("filas distintas")
	}
}

func TestScriptGeneratedColumnsAreSkipped(t *testing.T) {
	src := openSQLite(t, `CREATE TABLE g (a INT, b INT GENERATED ALWAYS AS (a * 2) VIRTUAL)`, `INSERT INTO g (a) VALUES (3)`)
	script, st := genScript(t, src, ScriptOptions{Mode: "both"})
	if len(st.Errors) != 0 {
		t.Fatal(st.Errors)
	}
	if strings.Contains(script, `"a", "b"`) {
		t.Errorf("no debe insertar la columna calculada:\n%s", script)
	}
	dst := openSQLite(t)
	if _, err := dst.Exec(script); err != nil {
		t.Fatalf("%v\n%s", err, script)
	}
	var b int
	if err := dst.QueryRow(`SELECT b FROM g`).Scan(&b); err != nil || b != 6 {
		t.Errorf("b=%d err=%v", b, err)
	}
}

func TestScriptBrokenObjectDoesNotStopTheRest(t *testing.T) {
	src := openSQLite(t, scriptFixture...)
	// Un objeto inexistente deja un comentario de error y el resto se genera.
	script, st := genScript(t, src, ScriptOptions{Mode: "both", Objects: []ScriptObject{
		{Name: "no_existe", Kind: "table"}, {Name: "z_padre", Kind: "table"}}})
	if len(st.Errors) == 0 || !strings.Contains(script, "-- ERROR: no_existe") {
		t.Errorf("falta el error de no_existe: %v\n%s", st.Errors, script)
	}
	if !strings.Contains(script, "CREATE TABLE z_padre") || st.Rows != 2 {
		t.Errorf("z_padre debía salir completa:\n%s", script)
	}
}

func TestScriptCancel(t *testing.T) {
	src := openSQLite(t, scriptFixture...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var sb strings.Builder
	if _, err := generateScript(ctx, src, mustDialect(t), "t", ScriptOptions{}, &sb, nil); err == nil {
		t.Error("una generación cancelada debe devolver error")
	}
}

func TestScriptRejectsUnknownMode(t *testing.T) {
	src := openSQLite(t)
	var sb strings.Builder
	if _, err := generateScript(context.Background(), src, mustDialect(t), "t", ScriptOptions{Mode: "x"}, &sb, nil); err == nil {
		t.Error("un modo desconocido debe fallar")
	}
}

func TestTopoOrder(t *testing.T) {
	deps := map[int][]int{0: {2}, 1: {0}, 2: nil} // 1 → 0 → 2
	got := topoOrder(3, func(i int) []int { return deps[i] })
	want := []int{2, 0, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("orden %v, se esperaba %v", got, want)
		}
	}
	// Con un ciclo termina y devuelve todos los nodos.
	cyc := map[int][]int{0: {1}, 1: {0}}
	if got := topoOrder(2, func(i int) []int { return cyc[i] }); len(got) != 2 {
		t.Fatalf("ciclo: %v", got)
	}
}

func TestDropStmt(t *testing.T) {
	cases := []struct {
		engine string
		obj    ScriptObject
		want   string
	}{
		{"sqlite", ScriptObject{Name: "t", Kind: "table"}, `DROP TABLE IF EXISTS "t"`},
		{"mysql", ScriptObject{Name: "p", Kind: "procedure"}, "DROP PROCEDURE IF EXISTS `p`"},
		{"postgres", ScriptObject{Schema: "s", Name: "t", Kind: "table"}, `DROP TABLE IF EXISTS "s"."t" CASCADE`},
		{"postgres", ScriptObject{Schema: "s", Name: "f", Kind: "function", Args: "a integer"}, `DROP FUNCTION IF EXISTS "s"."f"(a integer)`},
		{"mssql", ScriptObject{Schema: "dbo", Name: "t", Kind: "table"}, `IF OBJECT_ID(N'[dbo].[t]', N'U') IS NOT NULL DROP TABLE [dbo].[t]`},
		{"mssql", ScriptObject{Schema: "dbo", Name: "f", Kind: "function"}, `IF OBJECT_ID(N'[dbo].[f]') IS NOT NULL DROP FUNCTION [dbo].[f]`},
	}
	for _, c := range cases {
		d, err := driver.Get(c.engine)
		if err != nil {
			t.Fatal(err)
		}
		if got := dropStmt(d, c.obj); got != c.want {
			t.Errorf("%s: %q, se esperaba %q", c.engine, got, c.want)
		}
	}
}

func newScriptService(t *testing.T, db *sql.DB) *ScriptService {
	return NewScriptService(fakePools{db, mustDialect(t)}, &Events{})
}

func TestObjectScript(t *testing.T) {
	src := openSQLite(t, scriptFixture...)
	s := newScriptService(t, src)
	run := func(kind, name, action string) string {
		out, err := s.ObjectScript(ObjectScriptRequest{Profile: "p", Object: ScriptObject{Name: name, Kind: kind}, Action: action})
		if err != nil {
			t.Fatalf("%s %s: %v", name, action, err)
		}
		return out
	}
	if out := run("table", "z_padre", "create"); !strings.Contains(out, "CREATE TABLE z_padre") {
		t.Errorf("create: %s", out)
	}
	if out := run("table", "z_padre", "drop"); out != `DROP TABLE IF EXISTS "z_padre";` {
		t.Errorf("drop: %s", out)
	}
	if out := run("table", "z_padre", "select"); !strings.Contains(out, `"id"`) || !strings.Contains(out, `FROM "z_padre";`) {
		t.Errorf("select: %s", out)
	}
	if out := run("table", "a_hija", "insert"); !strings.Contains(out, `INSERT INTO "a_hija"`) || strings.Count(out, "NULL") != 2 || strings.Contains(out, `"id"`) {
		t.Errorf("insert: %s", out)
	}
	if out := run("view", "v_hijas", "select"); !strings.Contains(out, `FROM "v_hijas";`) {
		t.Errorf("select de vista: %s", out)
	}
	data := run("table", "z_padre", "data")
	if strings.Contains(data, "PRAGMA") || strings.Contains(data, "--  ") || strings.Count(data, "(") < 2 || !strings.Contains(data, "INSERT INTO") {
		t.Errorf("data: %s", data)
	}

	if _, err := s.ObjectScript(ObjectScriptRequest{Profile: "p", Object: ScriptObject{Name: "v_hijas", Kind: "view"}, Action: "data"}); err == nil {
		t.Error("las vistas no tienen datos que escribir")
	}
	if _, err := s.ObjectScript(ObjectScriptRequest{Profile: "p", Object: ScriptObject{Name: "x", Kind: "table"}, Action: "nope"}); err == nil {
		t.Error("una acción desconocida debe fallar")
	}
}

func TestObjectScriptDataIsLimited(t *testing.T) {
	src := openSQLite(t, `CREATE TABLE n (id INTEGER PRIMARY KEY)`)
	for i := 1; i <= 1500; i++ {
		src.Exec(`INSERT INTO n VALUES (?)`, i)
	}
	out, err := newScriptService(t, src).ObjectScript(ObjectScriptRequest{Profile: "p", Object: ScriptObject{Name: "n", Kind: "table"}, Action: "data"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out, "\n  ("); got != 1000 {
		t.Errorf("filas=%d, se esperaban 1000", got)
	}
}

func TestGenerateToFileAndMemory(t *testing.T) {
	src := openSQLite(t, scriptFixture...)
	s := newScriptService(t, src)

	mem, err := s.Generate("a", ScriptRequest{Profile: "p", Database: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mem.Script, "CREATE TABLE z_padre") || mem.Rows != 5 || mem.Path != "" {
		t.Errorf("memoria: %+v", mem)
	}

	path := filepath.Join(t.TempDir(), "out.sql")
	file, err := s.Generate("b", ScriptRequest{Profile: "p", Database: "t", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if file.Script != "" || file.Bytes == 0 || file.Rows != 5 {
		t.Errorf("archivo: %+v", file)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openSQLite(t).Exec(string(body)); err != nil {
		t.Errorf("el archivo generado no se ejecuta: %v", err)
	}
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Error("no debe quedar el .part")
	}

	if _, err := s.Generate("c", ScriptRequest{Profile: "p", Path: filepath.Join(t.TempDir(), "no", "x.sql")}); err == nil {
		t.Error("una carpeta inexistente debe fallar")
	}
}

func TestObjectsListsEverything(t *testing.T) {
	src := openSQLite(t, scriptFixture...)
	objs, err := newScriptService(t, src).Objects("p", "t")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, o := range objs {
		kinds[o.Kind]++
	}
	if kinds["table"] != 2 || kinds["view"] != 1 {
		t.Errorf("objetos: %+v", objs)
	}
}
