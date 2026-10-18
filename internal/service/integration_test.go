//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

// Ejecuta consultas reales y comprueba cómo se normalizan los tipos hacia JSON.
func TestIntegrationReadRows(t *testing.T) {
	cases := []struct {
		name  string
		p     config.ConnectionProfile
		query string
	}{
		{"postgres", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"},
			`SELECT id, email, edad, alta, 12.50::numeric AS precio, '\xff00'::bytea AS bin, 9007199254740993::bigint AS grande FROM clientes ORDER BY id`},
		{"mysql", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"},
			`SELECT id, email, edad, alta, 12.50 AS precio, bin, 9007199254740993 AS grande FROM clientes ORDER BY id`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			db, _, err := session.Open(ctx, tc.p, "pluton", "")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows, err := db.QueryContext(ctx, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			res := &QueryResult{Columns: []string{}, Rows: [][]any{}}
			if err := readRows(rows, res, 1); err != nil {
				t.Fatal(err)
			}
			t.Logf("columnas=%v truncado=%v", res.Columns, res.Truncated)
			for _, r := range res.Rows {
				for i, v := range r {
					t.Logf("  %-7s %T = %v", res.Columns[i], v, v)
				}
			}
			if !res.Truncated || len(res.Rows) != 1 {
				t.Errorf("el límite de filas no se aplicó: %d filas, truncado=%v", len(res.Rows), res.Truncated)
			}
			// Los valores deben ser serializables a JSON sin pérdida.
			if _, ok := res.Rows[0][len(res.Columns)-1].(string); !ok {
				t.Errorf("bigint > 2^53 debería ser string, es %T", res.Rows[0][len(res.Columns)-1])
			}
		})
	}
}

// Edición de datos contra motores reales: tipos estrictos, paginación y reglas de una sola fila.
func TestIntegrationDataEditing(t *testing.T) {
	cases := []struct {
		name   string
		schema string // PostgreSQL siempre trae esquema desde el árbol; MySQL no tiene
		p      config.ConnectionProfile
		setup  []string
	}{
		{"postgres", "public", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"}, []string{
			`DROP TABLE IF EXISTS data_test`, `DROP TABLE IF EXISTS data_nopk`,
			`CREATE TABLE data_test (id serial PRIMARY KEY, nombre text NOT NULL, edad int, alta timestamptz, activo boolean, saldo numeric(10,2))`,
			`CREATE TABLE data_nopk (a text, b int)`,
		}},
		{"mysql", "", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"}, []string{
			`DROP TABLE IF EXISTS data_test`, `DROP TABLE IF EXISTS data_nopk`,
			`CREATE TABLE data_test (id INT AUTO_INCREMENT PRIMARY KEY, nombre VARCHAR(50) NOT NULL, edad INT, alta DATETIME, activo TINYINT(1), saldo DECIMAL(10,2))`,
			`CREATE TABLE data_nopk (a VARCHAR(20), b INT)`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			db, d, err := session.Open(ctx, tc.p, "pluton", "")
			if err != nil {
				t.Fatalf("conectar (¿docker compose up -d --wait?): %v", err)
			}
			defer db.Close()
			for _, q := range tc.setup {
				if _, err := db.ExecContext(ctx, q); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
			}
			defer db.Exec(`DROP TABLE IF EXISTS data_test`)
			defer db.Exec(`DROP TABLE IF EXISTS data_nopk`)
			s := NewDataService(fakePools{db, d})
			at := func(tbl string) TableRef { return TableRef{Profile: "p", Schema: tc.schema, Table: tbl} }
			r := at("data_test")

			// Todo llega como texto desde el editor: el servidor debe convertirlo al tipo de la columna.
			var changes []Change
			for i := 1; i <= 12; i++ {
				changes = append(changes, Change{Kind: "insert", Values: map[string]any{
					"nombre": fmt.Sprintf("n%02d", i), "edad": fmt.Sprint(20 + i), "alta": "2026-01-02 03:04:05", "activo": "1", "saldo": "12.50"}})
			}
			if res, err := s.Apply(r, changes); err != nil || res.Inserted != 12 {
				t.Fatalf("insertar con texto: %+v %v", res, err)
			}

			p, err := s.Page(PageRequest{Table: r, Limit: 5})
			if err != nil || len(p.Rows) != 5 || !p.HasMore || !p.HasPK {
				t.Fatalf("página: %v filas=%d hasMore=%v", err, len(p.Rows), p != nil && p.HasMore)
			}
			p2, _ := s.Page(PageRequest{Table: r, Limit: 5, Offset: 10})
			if len(p2.Rows) != 2 || p2.HasMore {
				t.Errorf("última página: filas=%d hasMore=%v", len(p2.Rows), p2.HasMore)
			}
			desc, err := s.Page(PageRequest{Table: r, Limit: 1, OrderBy: "nombre", Desc: true})
			if err != nil || desc.Rows[0][1] != "n12" {
				t.Errorf("orden desc: %v %v", err, desc.Rows)
			}
			if n, err := s.Count(r, "edad > 30"); err != nil || n != 2 {
				t.Errorf("Count = %d, %v", n, err)
			}

			// Editar usando los valores que devolvió Page como clave (es lo que hace la interfaz).
			row := map[string]any{}
			for i, c := range p.Columns {
				row[c.Name] = p.Rows[0][i]
			}
			// Mismo valor: MySQL devuelve 0 filas modificadas salvo con ClientFoundRows.
			if _, err := s.Apply(r, []Change{{Kind: "update", Key: row, Values: map[string]any{"nombre": row["nombre"]}}}); err != nil {
				t.Errorf("update sin cambio real: %v", err)
			}
			if _, err := s.Apply(r, []Change{
				{Kind: "update", Key: row, Values: map[string]any{"edad": nil, "saldo": "99.99"}},
				{Kind: "delete", Key: map[string]any{"id": p.Rows[1][0]}},
			}); err != nil {
				t.Fatalf("update+delete: %v", err)
			}
			var edad sql.NullInt64
			var saldo string
			db.QueryRow(`SELECT edad, saldo FROM data_test WHERE nombre = 'n01'`).Scan(&edad, &saldo)
			if edad.Valid || saldo != "99.99" {
				t.Errorf("tras editar: edad=%v saldo=%s", edad, saldo)
			}

			// Sin clave primaria: filas idénticas se rechazan y se revierte todo.
			db.Exec(`INSERT INTO data_nopk VALUES ('dup', 1), ('dup', 1), ('solo', 2)`)
			if _, err := s.Apply(at("data_nopk"), []Change{
				{Kind: "update", Key: map[string]any{"a": "solo", "b": 2}, Values: map[string]any{"b": 3}},
				{Kind: "delete", Key: map[string]any{"a": "dup", "b": 1}},
			}); err == nil {
				t.Error("borrar una fila duplicada debería rechazarse")
			}
			var b int
			db.QueryRow(`SELECT b FROM data_nopk WHERE a = 'solo'`).Scan(&b)
			if b != 2 {
				t.Errorf("la transacción no se revirtió: b=%d", b)
			}
		})
	}
}
