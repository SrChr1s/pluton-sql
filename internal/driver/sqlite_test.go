package driver

import (
	"context"
	"database/sql"
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
		"mysql":    "u:p@ss@tcp(h:3306)/d?parseTime=true",
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
