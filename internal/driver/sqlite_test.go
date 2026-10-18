package driver

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

func TestSQLiteSchema(t *testing.T) {
	d, err := Get("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open(d.DriverName(), d.DSN(config.ConnectionProfile{FilePath: t.TempDir() + "/t.db"}, "", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, age INT DEFAULT 18)`,
		`CREATE UNIQUE INDEX ux_email ON users(email)`,
		`CREATE VIEW v AS SELECT * FROM users`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	tables, err := d.ListTables(ctx, db, "")
	if err != nil || len(tables) != 2 || tables[0].Name != "users" || tables[1].Type != "view" {
		t.Fatalf("tables: %+v %v", tables, err)
	}
	cols, err := d.ListColumns(ctx, db, "", "users")
	if err != nil || len(cols) != 3 || !cols[0].PrimaryKey || cols[1].Nullable || cols[2].Default != "18" {
		t.Fatalf("cols: %+v %v", cols, err)
	}
	idx, err := d.ListIndexes(ctx, db, "", "users")
	if err != nil || len(idx) != 1 || !idx[0].Unique || idx[0].Columns[0] != "email" {
		t.Fatalf("idx: %+v %v", idx, err)
	}
}

func TestDSNs(t *testing.T) {
	p := config.ConnectionProfile{Host: "h", User: "u", Database: "d"}
	want := map[string]string{
		"postgres": "postgres://u:p%40ss@h:5432/d?sslmode=prefer",
		"mysql":    "u:p@ss@tcp(h:3306)/d?clientFoundRows=true&parseTime=true",
	}
	for name, w := range want {
		d, _ := Get(name)
		pw := "p@ss"
		if got := d.DSN(p, pw, ""); got != w {
			t.Errorf("%s: got %q want %q", name, got, w)
		}
	}
}

func TestLegacyTLSDSN(t *testing.T) {
	p := config.ConnectionProfile{Host: "h", User: "u", TLSMinVersion: "1.0"}
	ms, _ := Get("mssql")
	if got := ms.DSN(p, "p", ""); !strings.Contains(got, "tlsmin=1.0") || !strings.Contains(got, "notraceid=true") {
		t.Errorf("mssql sin tlsmin/notraceid: %s", got)
	}
	my, _ := Get("mysql")
	if got := my.DSN(p, "p", ""); !strings.Contains(got, "tls=pluton-tls-1.0") {
		t.Errorf("mysql sin tls: %s", got)
	}
	pg, _ := Get("postgres")
	db, err := pg.(Opener).Open(p, "p", "")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	p.TLSMinVersion = ""
	if got := ms.DSN(p, "p", ""); strings.Contains(got, "tlsmin") {
		t.Errorf("tlsmin no debería estar: %s", got)
	}
}

func TestSQLiteOpenPaths(t *testing.T) {
	d, _ := Get("sqlite")
	open := d.(Opener).Open

	// Nombres con caracteres que romperían una URI sin escapar.
	for _, name := range []string{"con espacio.db", "almohadilla #1.db", "pregunta ?.db", "100% real.db", "ñandú.sqlite3"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, nil, 0o644); err != nil { // archivo vacío = base vacía
			t.Fatal(err)
		}
		db, err := open(config.ConnectionProfile{FilePath: path}, "", "")
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		if _, err := db.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		db.Close()
		// La base debe haberse escrito en ESE archivo y no en otro derivado del nombre.
		if st, err := os.Stat(path); err != nil || st.Size() == 0 {
			t.Errorf("%q: el archivo no recibió los datos (%v)", name, err)
		}
		if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
			t.Errorf("%q: se crearon archivos extra: %v", name, entries)
		}
	}

	missing := filepath.Join(t.TempDir(), "no-existe.db")
	cases := map[string]config.ConnectionProfile{
		"no existe":  {FilePath: missing},
		"es carpeta": {FilePath: t.TempDir()},
		"ruta vacía": {},
	}
	for name, p := range cases {
		if _, err := open(p, "", ""); err == nil {
			t.Errorf("%s: debería fallar", name)
		} else {
			t.Logf("%s → %v", name, err)
		}
	}
	if _, err := os.Stat(missing); err == nil {
		t.Error("abrir un archivo inexistente no debe crearlo")
	}
}

// OFFSET/FETCH solo existe desde SQL Server 2012; la paginación debe seguir funcionando en 2005/2008.
func TestMSSQLPagingAvoidsOffsetFetch(t *testing.T) {
	d, _ := Get("mssql")
	q, hidden := d.PageSQL("[dbo].[t]", " WHERE (a > 1)", "ORDER BY [id] ASC", 10, 20)
	for _, bad := range []string{"OFFSET", "FETCH"} {
		if strings.Contains(strings.ToUpper(q), bad) {
			t.Errorf("la consulta usa %s, no disponible antes de SQL Server 2012:\n%s", bad, q)
		}
	}
	if !strings.Contains(q, "ROW_NUMBER() OVER (ORDER BY [id] ASC)") || !strings.Contains(q, "__rn > 20 AND __rn <= 30") || hidden != 1 {
		t.Errorf("consulta inesperada (hidden=%d):\n%s", hidden, q)
	}
	if q2, _ := d.PageSQL("[t]", "", "", 5, 0); !strings.Contains(q2, "OVER (ORDER BY (SELECT NULL))") {
		t.Errorf("sin orden debe usar ORDER BY (SELECT NULL):\n%s", q2)
	}
}
