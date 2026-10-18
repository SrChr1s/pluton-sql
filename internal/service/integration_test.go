//go:build integration

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

// Ejecuta consultas reales y comprueba cómo se normalizan los tipos hacia JSON.
// mssqlPassword y optional: SQL Server es opcional (docker compose --profile mssql up -d --wait).
func passwordFor(engine string) string {
	if engine == "mssql" {
		return "Pluton_pass1"
	}
	return "pluton"
}

func openOrSkip(t *testing.T, ctx context.Context, engine string, p config.ConnectionProfile) (*sql.DB, driver.Dialect) {
	t.Helper()
	db, d, err := session.Open(ctx, p, passwordFor(engine), "")
	if err != nil {
		if engine == "mssql" {
			t.Skipf("SQL Server no está levantado (docker compose --profile mssql up -d --wait): %v", err)
		}
		t.Fatalf("conectar (¿docker compose up -d --wait?): %v", err)
	}
	return db, d
}

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
		{"mssql", "dbo", config.ConnectionProfile{Driver: "mssql", Host: "127.0.0.1", Port: 11433, User: "sa", Database: "master"}, []string{
			`DROP TABLE IF EXISTS data_test`, `DROP TABLE IF EXISTS data_nopk`,
			`CREATE TABLE data_test (id INT IDENTITY(1,1) PRIMARY KEY, nombre NVARCHAR(50) NOT NULL, edad INT, alta DATETIME2, activo BIT, saldo DECIMAL(10,2))`,
			`CREATE TABLE data_nopk (a NVARCHAR(20), b INT)`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			db, d := openOrSkip(t, ctx, tc.name, tc.p)
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

// El DDL reconstruido debe ser válido: se recrean las tablas con él y la estructura debe quedar igual.
func TestIntegrationStructureRoundtrip(t *testing.T) {
	cases := []struct {
		name, schema string
		p            config.ConnectionProfile
		setup        []string
		wantPadre    []string // tipos esperados de st_padre: id, codigo, nota
		wantHijoType string   // tipo de st_hijo.total
	}{
		{"postgres", "public", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"}, []string{
			`CREATE TABLE st_padre (id serial PRIMARY KEY, codigo varchar(20) NOT NULL, nota text)`,
			`CREATE UNIQUE INDEX ux_st_padre_codigo ON st_padre (codigo)`,
			`CREATE TABLE st_hijo (id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, padre_id int NOT NULL, total numeric(10,2) DEFAULT 0 NOT NULL,
				CONSTRAINT fk_hijo_padre FOREIGN KEY (padre_id) REFERENCES st_padre (id) ON DELETE CASCADE ON UPDATE RESTRICT, CONSTRAINT ck_total CHECK (total >= 0))`,
			`CREATE INDEX ix_st_hijo_padre_total ON st_hijo (padre_id, total)`,
			`CREATE VIEW st_vista AS SELECT id, codigo FROM st_padre`,
		}, []string{"integer", "character varying(20)", "text"}, "numeric(10,2)"},
		{"mysql", "", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"}, []string{
			`CREATE TABLE st_padre (id INT AUTO_INCREMENT PRIMARY KEY, codigo VARCHAR(20) NOT NULL, nota TEXT, UNIQUE KEY ux_st_padre_codigo (codigo))`,
			`CREATE TABLE st_hijo (id BIGINT AUTO_INCREMENT PRIMARY KEY, padre_id INT NOT NULL, total DECIMAL(10,2) NOT NULL DEFAULT 0,
				KEY ix_st_hijo_padre_total (padre_id, total),
				CONSTRAINT fk_hijo_padre FOREIGN KEY (padre_id) REFERENCES st_padre (id) ON DELETE CASCADE ON UPDATE RESTRICT)`,
			`CREATE VIEW st_vista AS SELECT id, codigo FROM st_padre`,
		}, []string{"int(11)", "varchar(20)", "text"}, "decimal(10,2)"},
		{"mssql", "dbo", config.ConnectionProfile{Driver: "mssql", Host: "127.0.0.1", Port: 11433, User: "sa", Database: "master"}, []string{
			`CREATE TABLE st_padre (id INT IDENTITY(1,1) PRIMARY KEY, codigo VARCHAR(20) NOT NULL, nota NVARCHAR(MAX))`,
			`CREATE UNIQUE INDEX ux_st_padre_codigo ON st_padre (codigo)`,
			`CREATE TABLE st_hijo (id BIGINT IDENTITY(1,1) PRIMARY KEY, padre_id INT NOT NULL, total DECIMAL(10,2) NOT NULL DEFAULT 0,
				CONSTRAINT fk_hijo_padre FOREIGN KEY (padre_id) REFERENCES st_padre (id) ON DELETE CASCADE ON UPDATE NO ACTION)`,
			`CREATE INDEX ix_st_hijo_padre_total ON st_hijo (padre_id, total)`,
			`CREATE VIEW st_vista AS SELECT id, codigo FROM st_padre`,
		}, []string{"int", "varchar(20)", "nvarchar(max)"}, "decimal(10,2)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			db, d := openOrSkip(t, ctx, tc.name, tc.p)
			defer db.Close()
			wantUpdate := "RESTRICT"
			if tc.name == "mssql" {
				wantUpdate = "NO ACTION" // SQL Server no tiene RESTRICT
			}
			drop := func() {
				for _, q := range []string{`DROP VIEW IF EXISTS st_vista`, `DROP TABLE IF EXISTS st_hijo`, `DROP TABLE IF EXISTS st_padre`} {
					if _, err := db.Exec(q); err != nil {
						t.Fatalf("%s: %v", q, err)
					}
				}
			}
			drop()
			defer drop()
			for _, q := range tc.setup {
				if _, err := db.ExecContext(ctx, q); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
			}
			svc := NewStructureService(fakePools{db, d})
			at := func(n string) TableRef { return TableRef{Profile: "p", Schema: tc.schema, Table: n} }

			padre, err := svc.Describe(at("st_padre"))
			if err != nil || padre.DDLError != "" {
				t.Fatalf("padre: %v %q", err, padre.DDLError)
			}
			for i, want := range tc.wantPadre {
				if padre.Columns[i].Type != want {
					t.Errorf("tipo de %s = %q, quería %q", padre.Columns[i].Name, padre.Columns[i].Type, want)
				}
			}
			if !padre.Columns[0].AutoIncrement || !padre.Columns[0].PrimaryKey || padre.Columns[1].AutoIncrement {
				t.Errorf("auto/pk de padre: %+v", padre.Columns)
			}
			hijo, err := svc.Describe(at("st_hijo"))
			if err != nil || hijo.DDLError != "" {
				t.Fatalf("hijo: %v %q", err, hijo.DDLError)
			}
			if hijo.Columns[2].Type != tc.wantHijoType {
				t.Errorf("tipo de total = %q, quería %q", hijo.Columns[2].Type, tc.wantHijoType)
			}
			if len(hijo.ForeignKeys) != 1 {
				t.Fatalf("fks: %+v", hijo.ForeignKeys)
			}
			if fk := hijo.ForeignKeys[0]; fk.Name != "fk_hijo_padre" || fk.RefTable != "st_padre" || fk.Columns[0] != "padre_id" || fk.RefColumns[0] != "id" || fk.OnDelete != "CASCADE" || fk.OnUpdate != wantUpdate {
				t.Errorf("fk: %+v", fk)
			}
			var primary, unique bool
			for _, ix := range hijo.Indexes {
				primary = primary || ix.Primary
			}
			for _, ix := range padre.Indexes {
				unique = unique || (ix.Name == "ux_st_padre_codigo" && ix.Unique && !ix.Primary)
			}
			if !primary || !unique {
				t.Errorf("índices: primary=%v unique=%v  %+v / %+v", primary, unique, hijo.Indexes, padre.Indexes)
			}
			vista, err := svc.Describe(at("st_vista"))
			if err != nil || !vista.IsView || vista.DDLError != "" || !strings.Contains(strings.ToUpper(vista.DDL), "VIEW") {
				t.Errorf("vista: %v %+v", err, vista)
			}

			// Ida y vuelta: recrear las tablas desde el DDL reconstruido.
			ddlPadre, ddlHijo := padre.DDL, hijo.DDL
			drop()
			for _, ddl := range []string{ddlPadre, ddlHijo} {
				if _, err := db.ExecContext(ctx, ddl); err != nil {
					t.Fatalf("el DDL reconstruido no se puede ejecutar: %v\n--- DDL ---\n%s", err, ddl)
				}
			}
			padre2, _ := svc.Describe(at("st_padre"))
			hijo2, _ := svc.Describe(at("st_hijo"))
			if !reflect.DeepEqual(padre.Columns, padre2.Columns) || !reflect.DeepEqual(hijo.Columns, hijo2.Columns) {
				t.Errorf("columnas distintas tras recrear:\n%+v\n%+v\n%+v\n%+v", padre.Columns, padre2.Columns, hijo.Columns, hijo2.Columns)
			}
			if !reflect.DeepEqual(hijo.ForeignKeys, hijo2.ForeignKeys) || !reflect.DeepEqual(hijo.Indexes, hijo2.Indexes) || !reflect.DeepEqual(padre.Indexes, padre2.Indexes) {
				t.Errorf("índices/claves distintos tras recrear:\n%+v\n%+v\n%+v\n%+v", hijo.ForeignKeys, hijo2.ForeignKeys, hijo.Indexes, hijo2.Indexes)
			}
		})
	}
}

// Ciclo de vida completo de modificaciones de estructura contra motores reales.
func TestIntegrationSchemaEditing(t *testing.T) {
	cases := []struct {
		name, schema string
		p            config.ConnectionProfile
		wantNota     string // tipo de la columna "nota" tras ampliarla a varchar(200)
	}{
		{"postgres", "public", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"}, "character varying(200)"},
		{"mysql", "", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"}, "varchar(200)"},
		{"mssql", "dbo", config.ConnectionProfile{Driver: "mssql", Host: "127.0.0.1", Port: 11433, User: "sa", Database: "master"}, "varchar(200)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			db, d := openOrSkip(t, ctx, tc.name, tc.p)
			defer db.Close()
			drop := func() {
				for _, tbl := range []string{"ed_t", "ed_t2", "ed_p", "ed_a"} {
					db.Exec("DROP TABLE IF EXISTS " + d.Quote(tbl))
				}
			}
			drop()
			defer drop()
			pools := fakePools{db, d}
			edit, desc := NewSchemaEditService(pools), NewStructureService(pools)
			at := func(tbl string) TableRef { return TableRef{Profile: "p", Schema: tc.schema, Table: tbl} }
			apply := func(tbl string, op SchemaOp) {
				t.Helper()
				if _, err := edit.Apply(at(tbl), op); err != nil {
					t.Fatalf("%s %s: %v", tbl, op.Kind, err)
				}
			}
			describe := func(tbl string) *TableStructure {
				t.Helper()
				st, err := desc.Describe(at(tbl))
				if err != nil {
					t.Fatalf("describe %s: %v", tbl, err)
				}
				return st
			}
			col := func(st *TableStructure, name string) (c driver.ColumnInfo, ok bool) {
				for _, x := range st.Columns {
					if x.Name == name {
						return x, true
					}
				}
				return
			}

			apply("ed_p", SchemaOp{Kind: "create_table", Columns: []ColumnDef{{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true}}})
			apply("ed_t", SchemaOp{Kind: "create_table", Columns: []ColumnDef{
				{Name: "id", Type: "int", PrimaryKey: true, AutoIncrement: true},
				{Name: "nombre", Type: "varchar(30)", Nullable: false},
				{Name: "nota", Type: "varchar(100)", Nullable: true},
			}})
			st := describe("ed_t")
			if id, _ := col(st, "id"); !id.PrimaryKey || !id.AutoIncrement {
				t.Errorf("create_table: id = %+v", id)
			}
			db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES ('uno')", d.Quote("ed_t"), d.Quote("nombre")))

			apply("ed_t", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "edad", Type: "int", Nullable: false, Default: "0"}})
			if _, ok := col(describe("ed_t"), "edad"); !ok {
				t.Fatal("add_column: no aparece la columna")
			}
			apply("ed_t", SchemaOp{Kind: "alter_column", Target: "nota", Column: &ColumnDef{Type: "varchar(200)", Nullable: true}})
			if c, _ := col(describe("ed_t"), "nota"); c.Type != tc.wantNota {
				t.Errorf("alter_column: tipo = %q, quería %q", c.Type, tc.wantNota)
			}
			apply("ed_t", SchemaOp{Kind: "alter_column", Target: "edad", Column: &ColumnDef{Type: "int", Nullable: true, Default: "7"}})
			if c, _ := col(describe("ed_t"), "edad"); !c.Nullable || !strings.Contains(c.Default, "7") {
				t.Errorf("alter_column nulabilidad/default: %+v", c)
			}
			apply("ed_t", SchemaOp{Kind: "alter_column", Target: "edad", Column: &ColumnDef{Type: "int", Nullable: true}}) // quitar el default
			if c, _ := col(describe("ed_t"), "edad"); c.Default != "" {
				t.Errorf("quitar default: %+v", c)
			}
			apply("ed_t", SchemaOp{Kind: "rename_column", Target: "nota", NewName: "comentario"})
			if _, ok := col(describe("ed_t"), "comentario"); !ok {
				t.Error("rename_column")
			}

			apply("ed_t", SchemaOp{Kind: "create_index", Index: &IndexDef{Name: "ix_ed_t_nombre_edad", Columns: []string{"nombre", "edad"}}})
			var found bool
			for _, ix := range describe("ed_t").Indexes {
				found = found || (ix.Name == "ix_ed_t_nombre_edad" && len(ix.Columns) == 2 && !ix.Unique)
			}
			if !found {
				t.Errorf("create_index: %+v", describe("ed_t").Indexes)
			}

			apply("ed_t", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "padre_id", Type: "int", Nullable: true}})
			fk := &FKDef{Name: "fk_ed_padre", Columns: []string{"padre_id"}, RefSchema: tc.schema, RefTable: "ed_p", RefColumns: []string{"id"}, OnDelete: "SET NULL"}
			if tc.name == "mysql" {
				fk.RefSchema = ""
			}
			apply("ed_t", SchemaOp{Kind: "add_fk", FK: fk})
			if fks := describe("ed_t").ForeignKeys; len(fks) != 1 || fks[0].Name != "fk_ed_padre" || fks[0].OnDelete != "SET NULL" || fks[0].RefTable != "ed_p" {
				t.Errorf("add_fk: %+v", fks)
			}
			apply("ed_t", SchemaOp{Kind: "drop_fk", Target: "fk_ed_padre"})
			if len(describe("ed_t").ForeignKeys) != 0 {
				t.Error("drop_fk")
			}
			apply("ed_t", SchemaOp{Kind: "drop_index", Target: "ix_ed_t_nombre_edad"})
			for _, ix := range describe("ed_t").Indexes {
				if ix.Name == "ix_ed_t_nombre_edad" {
					t.Error("drop_index")
				}
			}

			// Borrar una columna con valor por defecto: SQL Server exige quitar antes la restricción.
			apply("ed_t", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "tmp", Type: "int", Nullable: false, Default: "1"}})
			apply("ed_t", SchemaOp{Kind: "drop_column", Target: "tmp"})
			if _, ok := col(describe("ed_t"), "tmp"); ok {
				t.Error("drop_column")
			}

			apply("ed_t", SchemaOp{Kind: "rename_table", NewName: "ed_t2"})
			if _, err := desc.Describe(at("ed_t")); err == nil {
				t.Error("la tabla vieja no debería existir")
			}
			var n int
			db.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s", d.Quote("ed_t2"))).Scan(&n)
			if n != 1 {
				t.Errorf("los datos deben sobrevivir al renombrado: %d", n)
			}
			apply("ed_t2", SchemaOp{Kind: "drop_table"})
			apply("ed_p", SchemaOp{Kind: "drop_table"})
			if _, err := desc.Describe(at("ed_t2")); err == nil {
				t.Error("drop_table")
			}

			// Preview no ejecuta nada y avisa de lo destructivo.
			apply("ed_a", SchemaOp{Kind: "create_table", Columns: []ColumnDef{{Name: "n", Type: "varchar(10)", Nullable: true}}})
			p, err := edit.Preview(at("ed_a"), SchemaOp{Kind: "drop_table"})
			if err != nil || !p.Destructive || len(p.Warnings) == 0 {
				t.Errorf("preview: %v %+v", err, p)
			}
			describe("ed_a")
			if tc.name == "mysql" && p.Transactional {
				t.Error("MySQL debe avisar de que no es transaccional")
			}
		})
	}
}

// En PostgreSQL un ALTER de varias sentencias se revierte entero si una falla.
func TestIntegrationSchemaEditAtomicPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, d := openOrSkip(t, ctx, "postgres", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"})
	defer db.Close()
	db.Exec(`DROP TABLE IF EXISTS ed_atomic`)
	defer db.Exec(`DROP TABLE IF EXISTS ed_atomic`)
	for _, q := range []string{`CREATE TABLE ed_atomic (n varchar(10))`, `INSERT INTO ed_atomic VALUES ('1'), (NULL)`} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	pools := fakePools{db, d}
	ref := TableRef{Profile: "p", Schema: "public", Table: "ed_atomic"}
	// 1.ª sentencia (varchar → int) válida; 2.ª (SET NOT NULL) falla por el NULL.
	_, err := NewSchemaEditService(pools).Apply(ref, SchemaOp{Kind: "alter_column", Target: "n", Column: &ColumnDef{Type: "int", Nullable: false}})
	if err == nil || !strings.Contains(err.Error(), "ningún cambio") {
		t.Fatalf("debía fallar y avisar de que no se aplicó nada: %v", err)
	}
	st, _ := NewStructureService(pools).Describe(ref)
	if st.Columns[0].Type != "character varying(10)" {
		t.Errorf("el cambio de tipo no se revirtió: %q", st.Columns[0].Type)
	}
}

// Exportar y volver a importar contra motores reales: los datos deben quedar idénticos.
func TestIntegrationExportImportRoundtrip(t *testing.T) {
	cases := []struct {
		name, schema string
		p            config.ConnectionProfile
		ddl          string
	}{
		{"postgres", "public", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"},
			`CREATE TABLE xt (id int PRIMARY KEY, nombre varchar(50) NOT NULL, saldo numeric(10,2), activo boolean, alta timestamp, nota text)`},
		{"mysql", "", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"},
			`CREATE TABLE xt (id INT PRIMARY KEY, nombre VARCHAR(50) NOT NULL, saldo DECIMAL(10,2), activo TINYINT(1), alta DATETIME, nota TEXT)`},
		{"mssql", "dbo", config.ConnectionProfile{Driver: "mssql", Host: "127.0.0.1", Port: 11433, User: "sa", Database: "master"},
			`CREATE TABLE xt (id INT PRIMARY KEY, nombre NVARCHAR(50) NOT NULL, saldo DECIMAL(10,2), activo BIT, alta DATETIME2, nota NVARCHAR(MAX))`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			db, d := openOrSkip(t, ctx, tc.name, tc.p)
			defer db.Close()
			db.Exec(`DROP TABLE IF EXISTS xt`)
			defer db.Exec(`DROP TABLE IF EXISTS xt`)
			if _, err := db.Exec(tc.ddl); err != nil {
				t.Fatal(err)
			}
			pools := fakePools{db, d}
			ref := TableRef{Profile: "p", Schema: tc.schema, Table: "xt"}
			data := NewDataService(pools)
			ev := &Events{}
			exp, imp := NewExportService(pools, ev), NewImportService(pools, ev)
			dir := t.TempDir()

			// Valores difíciles: comillas, barra invertida, saltos de línea, Unicode, NULL y vacío.
			nombres := []string{"O'Brien", `a\b`, "línea1\nlínea2", "ñandú 日本語", "con \"comillas\", y coma", "último"}
			var ch []Change
			for i, n := range nombres {
				v := map[string]any{"id": fmt.Sprint(i + 1), "nombre": n, "saldo": fmt.Sprintf("%d.50", i*7), "activo": fmt.Sprint(i % 2), "alta": "2026-01-02 03:04:05", "nota": n}
				if i == 3 {
					delete(v, "saldo")
					delete(v, "nota") // NULL
				}
				ch = append(ch, Change{Kind: "insert", Values: v})
			}
			if _, err := data.Apply(ref, ch); err != nil {
				t.Fatalf("preparar datos: %v", err)
			}
			snapshot := func() [][]any {
				p, err := data.Page(PageRequest{Table: ref, OrderBy: "id", Limit: 100})
				if err != nil {
					t.Fatal(err)
				}
				return p.Rows
			}
			before := snapshot()
			if len(before) != len(nombres) {
				t.Fatalf("filas preparadas: %d", len(before))
			}
			wipe := func() {
				if _, err := db.Exec(`DELETE FROM xt`); err != nil {
					t.Fatal(err)
				}
			}
			req := func(format string) ExportRequest {
				return ExportRequest{Profile: "p", Kind: "table", Schema: tc.schema, Table: "xt", Format: format, Header: true, Path: filepath.Join(dir, "xt."+format)}
			}

			// --- JSON: válido y con todas las filas
			if res, err := exp.Run("j", req("json")); err != nil || res.Rows != int64(len(nombres)) {
				t.Fatalf("json: %v %+v", err, res)
			}
			var objs []map[string]any
			raw, _ := os.ReadFile(req("json").Path)
			if err := json.Unmarshal(raw, &objs); err != nil || len(objs) != len(nombres) || objs[0]["nombre"] != "O'Brien" {
				t.Errorf("JSON: %v", err)
			}

			// --- SQL: vaciar y recuperar ejecutando el script exportado
			if _, err := exp.Run("s", req("sql")); err != nil {
				t.Fatalf("sql: %v", err)
			}
			script, _ := os.ReadFile(req("sql").Path)
			wipe()
			stmts := strings.Split(string(script), ";\nINSERT INTO ") // las sentencias empiezan siempre por INSERT INTO
			for i, st := range stmts {
				if i > 0 {
					st = "INSERT INTO " + st
				}
				if _, err := db.ExecContext(ctx, strings.TrimSuffix(strings.TrimSpace(st), ";")); err != nil {
					t.Fatalf("el SQL exportado no se puede ejecutar: %v\n%s", err, st)
				}
			}
			if after := snapshot(); !reflect.DeepEqual(before, after) {
				t.Errorf("datos distintos tras recuperar desde el SQL:\nantes   %v\ndespués %v", before, after)
			}

			// --- CSV: vaciar y recuperar con la importación
			if res, err := exp.Run("c", req("csv")); err != nil || res.Rows != int64(len(nombres)) {
				t.Fatalf("csv: %v %+v", err, res)
			}
			wipe()
			mapping := []ImportMap{{"id", 0}, {"nombre", 1}, {"saldo", 2}, {"activo", 3}, {"alta", 4}, {"nota", 5}}
			res, err := imp.Run("i", ImportRequest{Table: ref, Path: req("csv").Path, HasHeader: true, Mapping: mapping})
			if err != nil || res.Inserted != int64(len(nombres)) {
				t.Fatalf("importar el CSV exportado: %v %+v", err, res)
			}
			after := snapshot()
			// La importación convierte "" en NULL en columnas no textuales; la nota NULL exportada como vacío
			// vuelve como '' (columna de texto): es la única diferencia admisible.
			for i := range before {
				for j := range before[i] {
					if !reflect.DeepEqual(before[i][j], after[i][j]) && !(before[i][j] == nil && after[i][j] == "") {
						t.Errorf("fila %d col %d: antes %#v, después %#v", i, j, before[i][j], after[i][j])
					}
				}
			}
		})
	}
}

// El índice de autocompletado debe traer tablas, vistas y columnas en orden, sin objetos del sistema.
func TestIntegrationListAllColumns(t *testing.T) {
	cases := []struct {
		name, schema string
		p            config.ConnectionProfile
		setup        []string
	}{
		{"postgres", "public", config.ConnectionProfile{Driver: "postgres", Host: "127.0.0.1", Port: 15432, User: "pluton", Database: "pluton", SSLMode: "disable"},
			[]string{`CREATE TABLE ac_t (id int, nombre text, activo boolean)`, `CREATE VIEW ac_v AS SELECT id FROM ac_t`}},
		{"mysql", "", config.ConnectionProfile{Driver: "mysql", Host: "127.0.0.1", Port: 13306, User: "pluton", Database: "pluton"},
			[]string{`CREATE TABLE ac_t (id INT, nombre VARCHAR(20), activo TINYINT(1))`, `CREATE VIEW ac_v AS SELECT id FROM ac_t`}},
		{"mssql", "dbo", config.ConnectionProfile{Driver: "mssql", Host: "127.0.0.1", Port: 11433, User: "sa", Database: "master"},
			[]string{`CREATE TABLE ac_t (id INT, nombre NVARCHAR(20), activo BIT)`, `CREATE VIEW ac_v AS SELECT id FROM ac_t`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			db, d := openOrSkip(t, ctx, tc.name, tc.p)
			defer db.Close()
			db.Exec(`DROP VIEW IF EXISTS ac_v`)
			db.Exec(`DROP TABLE IF EXISTS ac_t`)
			defer db.Exec(`DROP TABLE IF EXISTS ac_t`)
			defer db.Exec(`DROP VIEW IF EXISTS ac_v`)
			for _, q := range tc.setup {
				if _, err := db.Exec(q); err != nil {
					t.Fatal(err)
				}
			}
			refs, err := d.ListAllColumns(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			var tCols []string
			var viewSeen, tableSeen bool
			for _, r := range refs {
				switch r.Table {
				case "ac_t":
					tableSeen = !r.IsView
					tCols = append(tCols, r.Column)
					if r.Schema != tc.schema {
						t.Errorf("esquema = %q, quería %q", r.Schema, tc.schema)
					}
				case "ac_v":
					viewSeen = r.IsView
				}
				if r.Schema == "information_schema" || r.Schema == "pg_catalog" || r.Schema == "sys" || r.Schema == "INFORMATION_SCHEMA" {
					t.Errorf("objeto del sistema en el índice: %+v", r)
				}
			}
			if !tableSeen || !viewSeen || strings.Join(tCols, ",") != "id,nombre,activo" {
				t.Errorf("tabla=%v vista=%v columnas=%v", tableSeen, viewSeen, tCols)
			}
		})
	}
}
