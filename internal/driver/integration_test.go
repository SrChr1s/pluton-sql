//go:build integration

package driver_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

// Requiere `docker compose up -d --wait` (ver docker-compose.yml).
func TestIntegration(t *testing.T) {
	cases := []struct {
		name    string
		p       config.ConnectionProfile
		pw      string
		schema  string // esquema donde viven las tablas de prueba
		dbs     []string
		schemas []string
	}{
		{"postgres", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"},
			"pluton", "public", []string{"pluton", "postgres"}, []string{"public", "ventas"}},
		{"mysql", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"},
			"pluton", "", []string{"otra", "pluton"}, []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			db, d, err := session.Open(ctx, tc.p, tc.pw, "")
			if err != nil {
				t.Fatalf("conectar (¿docker compose up -d --wait?): %v", err)
			}
			defer db.Close()

			dbs, err := d.ListDatabases(ctx, db)
			if err != nil || !contains(dbs, tc.dbs) {
				t.Errorf("ListDatabases = %v, %v; quería incluir %v", dbs, err, tc.dbs)
			}
			schemas, err := d.ListSchemas(ctx, db)
			if err != nil || !contains(schemas, tc.schemas) {
				t.Errorf("ListSchemas = %v, %v; quería incluir %v", schemas, err, tc.schemas)
			}

			tables, err := d.ListTables(ctx, db, tc.schema)
			if err != nil {
				t.Fatal(err)
			}
			types := map[string]string{}
			for _, tb := range tables {
				types[tb.Name] = tb.Type
			}
			if types["clientes"] != "table" || types["v_clientes"] != "view" {
				t.Errorf("ListTables = %+v", tables)
			}

			cols, err := d.ListColumns(ctx, db, tc.schema, "clientes")
			if err != nil {
				t.Fatal(err)
			}
			byName := map[string]driver.ColumnInfo{}
			for _, c := range cols {
				byName[c.Name] = c
			}
			if len(cols) < 4 || cols[0].Name != "id" || !byName["id"].PrimaryKey || byName["email"].PrimaryKey {
				t.Errorf("columnas/PK incorrectas: %+v", cols)
			}
			if byName["email"].Nullable || !byName["edad"].Nullable {
				t.Errorf("nulabilidad incorrecta: email=%+v edad=%+v", byName["email"], byName["edad"])
			}
			if byName["edad"].Default != "18" {
				t.Errorf("default de edad = %q, quería 18", byName["edad"].Default)
			}

			idx, err := d.ListIndexes(ctx, db, tc.schema, "clientes")
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]driver.IndexInfo{}
			for _, i := range idx {
				got[i.Name] = i
			}
			if u := got["ux_clientes_email"]; !u.Unique || !reflect.DeepEqual(u.Columns, []string{"email"}) {
				t.Errorf("ux_clientes_email = %+v", u)
			}
			if i := got["ix_clientes_edad_alta"]; i.Unique || !reflect.DeepEqual(i.Columns, []string{"edad", "alta"}) {
				t.Errorf("ix_clientes_edad_alta = %+v (orden de columnas)", i)
			}

			var n int
			if err := db.QueryRowContext(ctx, "SELECT count(*) FROM clientes").Scan(&n); err != nil || n != 2 {
				t.Errorf("count = %d, %v", n, err)
			}
		})
	}
}

// contains informa si got incluye todos los elementos de want.
func contains(got, want []string) bool {
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func TestIntegrationCancelAndErrors(t *testing.T) {
	cases := []struct {
		name  string
		p     config.ConnectionProfile
		sleep string
	}{
		{"postgres", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"}, "SELECT pg_sleep(30)"},
		{"mysql", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"}, "SELECT SLEEP(30)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bg := context.Background()

			// Contraseña incorrecta: debe fallar rápido con error.
			if _, _, err := session.Open(bg, tc.p, "incorrecta", ""); err == nil {
				t.Error("con contraseña incorrecta debería fallar")
			} else {
				t.Logf("contraseña incorrecta → %v", err)
			}
			// Puerto cerrado.
			bad := tc.p
			bad.Port = 1
			if _, _, err := session.Open(bg, bad, "pluton", ""); err == nil {
				t.Error("con puerto cerrado debería fallar")
			}

			db, _, err := session.Open(bg, tc.p, "pluton", "")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			// Cancelar una consulta de 30 s a los 300 ms.
			ctx, cancel := context.WithCancel(bg)
			time.AfterFunc(300*time.Millisecond, cancel)
			start := time.Now()
			rows, err := db.QueryContext(ctx, tc.sleep)
			if err == nil {
				for rows.Next() {
				}
				err = rows.Err()
				rows.Close()
			}
			took := time.Since(start)
			if err == nil || took > 5*time.Second {
				t.Errorf("la cancelación no cortó la consulta: err=%v tras %v", err, took)
			}
			t.Logf("cancelada tras %v: %v", took.Round(time.Millisecond), err)

			// Tras cancelar, el pool debe seguir usable.
			var one int
			if err := db.QueryRowContext(bg, "SELECT 1").Scan(&one); err != nil || one != 1 {
				t.Errorf("el pool quedó inutilizable tras cancelar: %v", err)
			}
		})
	}
}
