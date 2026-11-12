//go:build integration

package service

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

// splitMySQL parte un script de MySQL en sentencias, respetando DELIMITER (que es del cliente, no del servidor).
func splitMySQL(script string) []string {
	var out []string
	var cur []string
	delim := ";"
	flush := func() {
		body := strings.TrimSpace(strings.Join(cur, "\n"))
		cur = nil
		code := false
		for _, l := range strings.Split(body, "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "--") {
				code = true
			}
		}
		if code {
			out = append(out, strings.TrimSpace(strings.TrimSuffix(body, delim)))
		}
	}
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(strings.ToUpper(line), "DELIMITER ") {
			delim = strings.TrimSpace(line[len("DELIMITER "):])
			continue
		}
		cur = append(cur, line)
		if strings.HasSuffix(strings.TrimSpace(line), delim) {
			flush()
		}
	}
	flush()
	return out
}

// dump lee una consulta como texto para comparar los datos antes y después de restaurar.
func dump(t *testing.T, db *sql.DB, q string) []string {
	t.Helper()
	rows, err := db.Query(q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				v = "0x" + hex.EncodeToString(b)
			}
			parts[i] = fmt.Sprint(v)
		}
		out = append(out, strings.Join(parts, "|"))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Genera el script de unas tablas reales, lo ejecuta (con y sin objetos previos) y comprueba que los datos
// vuelven idénticos y que las secuencias/identidades siguen donde deben.
func TestIntegrationScriptRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		schema string
		p      config.ConnectionProfile
		setup  []string
		drop   []string
		// compare son consultas con columnas explícitas (rowversion cambia al restaurar).
		compare []string
		// run ejecuta el script entero.
		run func(t *testing.T, ctx context.Context, db *sql.DB, script string)
		// next inserta filas sin dar el id: debe funcionar y no chocar.
		next    []string
		expects []string // fragmentos que debe contener el script
	}{
		{
			name: "postgres", schema: "public",
			p:    config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"},
			drop: []string{`DROP VIEW IF EXISTS scr_v`, `DROP TABLE IF EXISTS scr_child`, `DROP TABLE IF EXISTS scr_parent`, `DROP FUNCTION IF EXISTS scr_f(int)`},
			setup: []string{
				`CREATE TABLE scr_parent (id serial PRIMARY KEY, nombre text NOT NULL, creado timestamptz, saldo numeric(10,2), activo boolean, doble int GENERATED ALWAYS AS (id * 2) STORED)`,
				`CREATE TABLE scr_child (id int GENERATED ALWAYS AS IDENTITY PRIMARY KEY, parent_id int NOT NULL REFERENCES scr_parent(id) ON DELETE CASCADE, nota text, bin bytea)`,
				`CREATE INDEX scr_child_parent ON scr_child (parent_id)`,
				`CREATE VIEW scr_v AS SELECT c.id, p.nombre FROM scr_child c JOIN scr_parent p ON p.id = c.parent_id`,
				`CREATE FUNCTION scr_f(a int) RETURNS int LANGUAGE sql AS $$ SELECT a + 1 $$`,
				`INSERT INTO scr_parent (nombre, creado, saldo, activo) VALUES ('uno', '2024-01-02 03:04:05+00', 12.50, true), ('dos ''q'' ñ \ x', NULL, NULL, false)`,
				`INSERT INTO scr_child (parent_id, nota, bin) VALUES (1, 'x', '\xff00'), (2, NULL, NULL), (2, 'línea1
línea2', NULL)`,
			},
			compare: []string{`SELECT id, nombre, creado, saldo, activo, doble FROM scr_parent ORDER BY id`, `SELECT id, parent_id, nota, bin FROM scr_child ORDER BY id`, `SELECT * FROM scr_v ORDER BY id`, `SELECT scr_f(1)`},
			run: func(t *testing.T, ctx context.Context, db *sql.DB, script string) {
				if _, err := db.ExecContext(ctx, script); err != nil {
					t.Fatalf("ejecutar el script: %v", err)
				}
			},
			next:    []string{`INSERT INTO scr_parent (nombre) VALUES ('nuevo')`, `INSERT INTO scr_child (parent_id) VALUES (1)`},
			expects: []string{"OVERRIDING SYSTEM VALUE", "setval(", "DROP TABLE IF EXISTS", "ADD CONSTRAINT"},
		},
		{
			name: "mysql", schema: "",
			p:    config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"},
			drop: []string{`DROP VIEW IF EXISTS scr_v`, `DROP TABLE IF EXISTS scr_child`, `DROP TABLE IF EXISTS scr_parent`, `DROP PROCEDURE IF EXISTS scr_p`},
			setup: []string{
				`CREATE TABLE scr_parent (id INT AUTO_INCREMENT PRIMARY KEY, nombre VARCHAR(50) NOT NULL, creado DATETIME, saldo DECIMAL(10,2), activo TINYINT(1), doble INT AS (id * 2) VIRTUAL)`,
				`CREATE TABLE scr_child (id INT AUTO_INCREMENT PRIMARY KEY, parent_id INT NOT NULL, nota TEXT, bin BLOB, KEY scr_child_parent (parent_id), CONSTRAINT scr_fk FOREIGN KEY (parent_id) REFERENCES scr_parent(id))`,
				`CREATE VIEW scr_v AS SELECT c.id, p.nombre FROM scr_child c JOIN scr_parent p ON p.id = c.parent_id`,
				`CREATE PROCEDURE scr_p() BEGIN SELECT 1; SELECT 2; END`,
				"INSERT INTO scr_parent (nombre, creado, saldo, activo) VALUES ('uno', '2024-01-02 03:04:05', 12.50, 1), ('dos ''q'' ñ \\\\ x', NULL, NULL, 0)",
				`INSERT INTO scr_child (parent_id, nota, bin) VALUES (1, 'x', 0xff00), (2, NULL, NULL), (2, 'línea1
línea2', NULL)`,
			},
			compare: []string{`SELECT id, nombre, creado, saldo, activo, doble FROM scr_parent ORDER BY id`, `SELECT id, parent_id, nota, bin FROM scr_child ORDER BY id`, `SELECT * FROM scr_v ORDER BY id`},
			run: func(t *testing.T, ctx context.Context, db *sql.DB, script string) {
				for _, stmt := range splitMySQL(script) {
					if _, err := db.ExecContext(ctx, stmt); err != nil {
						t.Fatalf("ejecutar %q: %v", stmt, err)
					}
				}
			},
			next:    []string{`INSERT INTO scr_parent (nombre) VALUES ('nuevo')`, `INSERT INTO scr_child (parent_id) VALUES (1)`},
			expects: []string{"FOREIGN_KEY_CHECKS=0", "DELIMITER $$", "DROP TABLE IF EXISTS"},
		},
		{
			name: "mssql", schema: "dbo",
			p:    config.ConnectionProfile{Driver: "mssql", Host: "127.0.0.1", Port: 11433, User: "sa", Database: "master"},
			drop: []string{`DROP VIEW IF EXISTS dbo.scr_v`, `DROP TABLE IF EXISTS dbo.scr_child`, `DROP TABLE IF EXISTS dbo.scr_parent`, `DROP FUNCTION IF EXISTS dbo.scr_f`},
			setup: []string{
				`CREATE TABLE dbo.scr_parent (id INT IDENTITY(1,1) PRIMARY KEY, nombre NVARCHAR(50) NOT NULL, creado DATETIME2, saldo DECIMAL(10,2), activo BIT, doble AS (id * 2) PERSISTED, rv ROWVERSION, uid UNIQUEIDENTIFIER)`,
				`CREATE TABLE dbo.scr_child (id INT IDENTITY(5,1) PRIMARY KEY, parent_id INT NOT NULL REFERENCES dbo.scr_parent(id), nota NVARCHAR(MAX), bin VARBINARY(10))`,
				`CREATE INDEX scr_child_parent ON dbo.scr_child (parent_id)`,
				`CREATE VIEW dbo.scr_v AS SELECT c.id, p.nombre FROM dbo.scr_child c JOIN dbo.scr_parent p ON p.id = c.parent_id`,
				`CREATE FUNCTION dbo.scr_f(@a INT) RETURNS INT AS BEGIN RETURN @a + 1 END`,
				`INSERT INTO dbo.scr_parent (nombre, creado, saldo, activo, uid) VALUES (N'uno', '2024-01-02 03:04:05', 12.50, 1, '6F9619FF-8B86-D011-B42D-00C04FC964FF'), (N'dos ''q'' ñ \ x', NULL, NULL, 0, NULL)`,
				`INSERT INTO dbo.scr_child (parent_id, nota, bin) VALUES (1, N'x', 0xff00), (2, NULL, NULL), (2, N'línea1
línea2', NULL)`,
			},
			compare: []string{`SELECT id, nombre, creado, saldo, activo, doble, CAST(uid AS varchar(36)) FROM dbo.scr_parent ORDER BY id`, `SELECT id, parent_id, nota, bin FROM dbo.scr_child ORDER BY id`, `SELECT * FROM dbo.scr_v ORDER BY id`, `SELECT dbo.scr_f(1)`},
			run: func(t *testing.T, ctx context.Context, db *sql.DB, script string) {
				for _, b := range splitGoBatches(script) {
					if _, err := db.ExecContext(ctx, b.sql); err != nil {
						t.Fatalf("ejecutar lote %q: %v", b.sql, err)
					}
				}
			},
			next:    []string{`INSERT INTO dbo.scr_parent (nombre) VALUES (N'nuevo')`, `INSERT INTO dbo.scr_child (parent_id) VALUES (1)`},
			expects: []string{"SET IDENTITY_INSERT [dbo].[scr_child] ON;", "\nGO\n", "IF OBJECT_ID(N'[dbo].[scr_parent]', N'U') IS NOT NULL DROP TABLE"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			db, d := openOrSkip(t, ctx, tc.name, tc.p)
			defer db.Close()
			cleanup := func() {
				for _, q := range tc.drop {
					db.Exec(q)
				}
			}
			cleanup()
			t.Cleanup(cleanup)
			for _, q := range tc.setup {
				if _, err := db.ExecContext(ctx, q); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
			}
			before := make([][]string, len(tc.compare))
			for i, q := range tc.compare {
				before[i] = dump(t, db, q)
			}

			all, err := listScriptObjects(ctx, db, d)
			if err != nil {
				t.Fatal(err)
			}
			var objs []ScriptObject
			for _, o := range all {
				if strings.HasPrefix(o.Name, "scr_") {
					objs = append(objs, o)
				}
			}
			if len(objs) != 4 { // dos tablas, una vista y una rutina
				t.Fatalf("objetos de prueba: %+v", objs)
			}
			var sb strings.Builder
			st, err := generateScript(ctx, db, d, tc.p.Database,
				ScriptOptions{Objects: objs, Mode: "both", IncludeDrop: true, IncludeSchemas: true, IncludeIndexes: true, IncludeFKs: true}, &sb, nil)
			if err != nil {
				t.Fatal(err)
			}
			script := sb.String()
			if len(st.Errors) != 0 {
				t.Fatalf("errores al generar: %v\n%s", st.Errors, script)
			}
			if st.Rows != 5 {
				t.Errorf("filas escritas = %d, se esperaban 5", st.Rows)
			}
			for _, want := range tc.expects {
				if !strings.Contains(script, want) {
					t.Errorf("el script no contiene %q", want)
				}
			}
			t.Logf("script (%d bytes):\n%s", len(script), script)

			check := func(label string) {
				t.Helper()
				for i, q := range tc.compare {
					if got := dump(t, db, q); strings.Join(got, "\n") != strings.Join(before[i], "\n") {
						t.Errorf("%s: %s\n  antes:   %q\n  después: %q", label, q, before[i], got)
					}
				}
			}
			// 1. Con los objetos todavía presentes: el DROP del script debe dejarlos recrear.
			tc.run(t, ctx, db, script)
			check("sobre objetos existentes")
			// 2. Sobre una base sin ellos.
			cleanup()
			tc.run(t, ctx, db, script)
			check("sobre una base vacía")

			// Las secuencias e identidades siguen detrás del máximo: insertar sin id no choca.
			for _, q := range tc.next {
				if _, err := db.ExecContext(ctx, q); err != nil {
					t.Errorf("tras restaurar, %s: %v", q, err)
				}
			}
		})
	}
}
