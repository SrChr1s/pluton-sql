//go:build integration

package service

import (
	"context"
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
