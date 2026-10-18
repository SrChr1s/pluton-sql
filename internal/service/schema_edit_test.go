package service

import (
	"strings"
	"testing"

	"github.com/SrChr1s/pluton-sql/internal/driver"
)

func dialect(t *testing.T, name string) driver.Dialect {
	t.Helper()
	d, err := driver.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

var cols = []driver.ColumnInfo{
	{Name: "id", Type: "int", PrimaryKey: true},
	{Name: "nombre", Type: "varchar(20)", Nullable: false},
	{Name: "nota", Type: "text", Nullable: true, Default: ""},
}

func plan(t *testing.T, engine string, ref TableRef, op SchemaOp) *SchemaPlan {
	t.Helper()
	p, err := buildPlan(dialect(t, engine), ref, op, cols, false)
	if err != nil {
		t.Fatalf("%s %s: %v", engine, op.Kind, err)
	}
	return p
}

func joined(p *SchemaPlan) string { return strings.Join(p.Statements, "\n") }

func TestBuildPlanPerDialect(t *testing.T) {
	pg, my, ms, lite := TableRef{Schema: "public", Table: "t"}, TableRef{Table: "t"}, TableRef{Schema: "dbo", Table: "t"}, TableRef{Table: "t"}
	col := &ColumnDef{Name: "edad", Type: "int", Nullable: false, Default: "0"}

	cases := []struct {
		engine string
		ref    TableRef
		op     SchemaOp
		want   []string
	}{
		{"postgres", pg, SchemaOp{Kind: "add_column", Column: col}, []string{`ALTER TABLE "public"."t" ADD COLUMN "edad" int NOT NULL DEFAULT 0`}},
		{"mysql", my, SchemaOp{Kind: "add_column", Column: col}, []string{"ALTER TABLE `t` ADD COLUMN `edad` int NOT NULL DEFAULT 0"}},
		{"mssql", ms, SchemaOp{Kind: "add_column", Column: col}, []string{"ALTER TABLE [dbo].[t] ADD [edad] int NOT NULL DEFAULT 0"}}, // sin COLUMN
		{"sqlite", lite, SchemaOp{Kind: "add_column", Column: col}, []string{`ALTER TABLE "t" ADD COLUMN "edad" int NOT NULL DEFAULT 0`}},

		{"postgres", pg, SchemaOp{Kind: "rename_column", Target: "nota", NewName: "n"}, []string{`RENAME COLUMN "nota" TO "n"`}},
		{"mssql", ms, SchemaOp{Kind: "rename_column", Target: "nota", NewName: "n"}, []string{`EXEC sp_rename N'[dbo].[t].[nota]', N'n', N'COLUMN'`}},

		{"postgres", pg, SchemaOp{Kind: "drop_column", Target: "nota"}, []string{`DROP COLUMN "nota"`}},
		{"mssql", ms, SchemaOp{Kind: "drop_column", Target: "nota"}, []string{"sys.default_constraints", "DROP COLUMN [nota]"}},

		{"postgres", pg, SchemaOp{Kind: "alter_column", Target: "nombre", Column: &ColumnDef{Type: "varchar(50)", Nullable: true}},
			[]string{`ALTER COLUMN "nombre" TYPE varchar(50) USING "nombre"::varchar(50)`, `ALTER COLUMN "nombre" DROP NOT NULL`}},
		{"mysql", my, SchemaOp{Kind: "alter_column", Target: "nombre", Column: &ColumnDef{Type: "varchar(50)", Nullable: true}}, []string{"MODIFY COLUMN `nombre` varchar(50)"}},
		{"mssql", ms, SchemaOp{Kind: "alter_column", Target: "nombre", Column: &ColumnDef{Type: "varchar(50)", Nullable: true, Default: "'x'"}},
			[]string{"ALTER COLUMN [nombre] varchar(50) NULL", "ADD DEFAULT 'x' FOR [nombre]"}},

		{"postgres", pg, SchemaOp{Kind: "create_index", Index: &IndexDef{Name: "ix", Columns: []string{"nombre", "nota"}, Unique: true}}, []string{`CREATE UNIQUE INDEX "ix" ON "public"."t" ("nombre", "nota")`}},
		{"postgres", pg, SchemaOp{Kind: "drop_index", Target: "ix"}, []string{`DROP INDEX "public"."ix"`}},
		{"mysql", my, SchemaOp{Kind: "drop_index", Target: "ix"}, []string{"DROP INDEX `ix` ON `t`"}},
		{"mssql", ms, SchemaOp{Kind: "drop_index", Target: "ix"}, []string{"DROP INDEX [ix] ON [dbo].[t]"}},
		{"sqlite", lite, SchemaOp{Kind: "drop_index", Target: "ix"}, []string{`DROP INDEX "ix"`}},

		{"postgres", pg, SchemaOp{Kind: "add_fk", FK: &FKDef{Name: "fk", Columns: []string{"id"}, RefSchema: "public", RefTable: "o", RefColumns: []string{"id"}, OnDelete: "CASCADE"}},
			[]string{`ADD CONSTRAINT "fk" FOREIGN KEY ("id") REFERENCES "public"."o" ("id") ON DELETE CASCADE`}},
		{"mysql", my, SchemaOp{Kind: "drop_fk", Target: "fk"}, []string{"DROP FOREIGN KEY `fk`"}},
		{"mssql", ms, SchemaOp{Kind: "drop_fk", Target: "fk"}, []string{"DROP CONSTRAINT [fk]"}},
		{"postgres", pg, SchemaOp{Kind: "drop_fk", Target: "fk"}, []string{`DROP CONSTRAINT "fk"`}},

		{"postgres", pg, SchemaOp{Kind: "rename_table", NewName: "u"}, []string{`ALTER TABLE "public"."t" RENAME TO "u"`}},
		{"mysql", my, SchemaOp{Kind: "rename_table", NewName: "u"}, []string{"RENAME TABLE `t` TO `u`"}},
		{"mssql", ms, SchemaOp{Kind: "rename_table", NewName: "u"}, []string{`EXEC sp_rename N'[dbo].[t]', N'u'`}},
		{"postgres", pg, SchemaOp{Kind: "drop_table"}, []string{`DROP TABLE "public"."t"`}},
	}
	for _, c := range cases {
		p := plan(t, c.engine, c.ref, c.op)
		got := joined(p)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s %s: falta %q en:\n%s", c.engine, c.op.Kind, w, got)
			}
		}
	}

	// Las operaciones destructivas se marcan como tales; las demás no.
	if !plan(t, "postgres", pg, SchemaOp{Kind: "drop_table"}).Destructive || !plan(t, "postgres", pg, SchemaOp{Kind: "drop_column", Target: "nota"}).Destructive {
		t.Error("drop_table y drop_column deben ser destructivas")
	}
	if plan(t, "postgres", pg, SchemaOp{Kind: "add_column", Column: col}).Destructive {
		t.Error("add_column no es destructiva")
	}
	// Solo MySQL carece de DDL transaccional.
	if plan(t, "mysql", my, SchemaOp{Kind: "add_column", Column: col}).Transactional || !plan(t, "postgres", pg, SchemaOp{Kind: "add_column", Column: col}).Transactional {
		t.Error("Transactional incorrecto")
	}
}

func TestBuildPlanCreateTable(t *testing.T) {
	defs := []ColumnDef{
		{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
		{Name: "nombre", Type: "varchar(30)", Nullable: false},
		{Name: "nota", Type: "text", Nullable: true, Default: "'-'"},
	}
	want := map[string][]string{
		"postgres": {`CREATE TABLE "public"."n"`, `"id" INTEGER GENERATED BY DEFAULT AS IDENTITY NOT NULL`, `PRIMARY KEY ("id")`},
		"mysql":    {"CREATE TABLE `n`", "`id` INTEGER AUTO_INCREMENT NOT NULL", "PRIMARY KEY (`id`)"},
		"mssql":    {"CREATE TABLE [dbo].[n]", "[id] INTEGER IDENTITY(1,1) NOT NULL", "[nota] text NULL DEFAULT '-'", "PRIMARY KEY ([id])"},
		"sqlite":   {`CREATE TABLE "n"`, `"id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL`},
	}
	refs := map[string]TableRef{"postgres": {Schema: "public", Table: "n"}, "mysql": {Table: "n"}, "mssql": {Schema: "dbo", Table: "n"}, "sqlite": {Table: "n"}}
	for engine, ws := range want {
		p, err := buildPlan(dialect(t, engine), refs[engine], SchemaOp{Kind: "create_table", Columns: defs}, nil, false)
		if err != nil {
			t.Fatalf("%s: %v", engine, err)
		}
		for _, w := range ws {
			if !strings.Contains(p.Statements[0], w) {
				t.Errorf("%s: falta %q en:\n%s", engine, w, p.Statements[0])
			}
		}
	}
	// SQLite: PRIMARY KEY inline, nunca duplicado como restricción de tabla.
	p, _ := buildPlan(dialect(t, "sqlite"), refs["sqlite"], SchemaOp{Kind: "create_table", Columns: defs}, nil, false)
	if strings.Count(p.Statements[0], "PRIMARY KEY") != 1 {
		t.Errorf("sqlite: PRIMARY KEY duplicado:\n%s", p.Statements[0])
	}
}

func TestBuildPlanRejectsBadInput(t *testing.T) {
	r := TableRef{Schema: "public", Table: "t"}
	bad := map[string]struct {
		engine string
		op     SchemaOp
	}{
		"tipo con inyección":       {"postgres", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "x", Type: "int); DROP TABLE t; --", Nullable: true}}},
		"default con ';'":          {"postgres", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "x", Type: "int", Nullable: true, Default: "1; DROP TABLE t"}}},
		"default con comentario":   {"postgres", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "x", Type: "int", Nullable: true, Default: "1 -- x"}}},
		"columna repetida":         {"postgres", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "nombre", Type: "int", Nullable: true}}},
		"nombre vacío":             {"postgres", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "  ", Type: "int", Nullable: true}}},
		"sqlite NOT NULL sin def.": {"sqlite", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "x", Type: "int", Nullable: false}}},
		"sqlite alter":             {"sqlite", SchemaOp{Kind: "alter_column", Target: "nombre", Column: &ColumnDef{Type: "text", Nullable: true}}},
		"sqlite add_fk":            {"sqlite", SchemaOp{Kind: "add_fk", FK: &FKDef{Name: "f", Columns: []string{"id"}, RefTable: "o", RefColumns: []string{"id"}}}},
		"alter sin cambios":        {"postgres", SchemaOp{Kind: "alter_column", Target: "nombre", Column: &ColumnDef{Type: "varchar(20)", Nullable: false}}},
		"borrar columna ajena":     {"postgres", SchemaOp{Kind: "drop_column", Target: "no_existe"}},
		"índice sin columnas":      {"postgres", SchemaOp{Kind: "create_index", Index: &IndexDef{Name: "ix"}}},
		"índice con columna falsa": {"postgres", SchemaOp{Kind: "create_index", Index: &IndexDef{Name: "ix", Columns: []string{"zzz"}}}},
		"fk con distinto número":   {"postgres", SchemaOp{Kind: "add_fk", FK: &FKDef{Name: "f", Columns: []string{"id", "nombre"}, RefTable: "o", RefColumns: []string{"id"}}}},
		"fk acción inválida":       {"postgres", SchemaOp{Kind: "add_fk", FK: &FKDef{Name: "f", Columns: []string{"id"}, RefTable: "o", RefColumns: []string{"id"}, OnDelete: "EXPLODE"}}},
		"operación desconocida":    {"postgres", SchemaOp{Kind: "truncate"}},
		"renombrar al mismo":       {"postgres", SchemaOp{Kind: "rename_table", NewName: "t"}},
		"tabla sin columnas":       {"postgres", SchemaOp{Kind: "create_table"}},
	}
	for name, c := range bad {
		if _, err := buildPlan(dialect(t, c.engine), r, c.op, cols, false); err == nil {
			t.Errorf("%s: debería rechazarse", name)
		}
	}
}

func TestSchemaEditLifecycleSQLite(t *testing.T) {
	svc, db := newData(t)
	edit := NewSchemaEditService(fakePools{db, mustDialect(t)})
	desc := NewStructureService(fakePools{db, mustDialect(t)})
	apply := func(table string, op SchemaOp) {
		t.Helper()
		if _, err := edit.Apply(ref(table), op); err != nil {
			t.Fatalf("%s: %v", op.Kind, err)
		}
	}
	colNames := func(table string) []string {
		st, err := desc.Describe(ref(table))
		if err != nil {
			t.Fatalf("describe %s: %v", table, err)
		}
		var n []string
		for _, c := range st.Columns {
			n = append(n, c.Name)
		}
		return n
	}
	_ = svc

	apply("ed", SchemaOp{Kind: "create_table", Columns: []ColumnDef{
		{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
		{Name: "nombre", Type: "TEXT", Nullable: false, Default: "'sin nombre'"},
	}})
	st, _ := desc.Describe(ref("ed"))
	if !st.Columns[0].PrimaryKey || !st.Columns[0].AutoIncrement || st.Columns[1].Nullable {
		t.Fatalf("create_table: %+v", st.Columns)
	}
	db.Exec(`INSERT INTO ed (nombre) VALUES ('a'), ('b')`)

	apply("ed", SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "edad", Type: "INT", Nullable: false, Default: "7"}})
	var edad int
	db.QueryRow(`SELECT edad FROM ed WHERE id = 1`).Scan(&edad)
	if edad != 7 {
		t.Errorf("las filas existentes deben tomar el valor por defecto: edad=%d", edad)
	}
	apply("ed", SchemaOp{Kind: "rename_column", Target: "nombre", NewName: "titulo"})
	apply("ed", SchemaOp{Kind: "create_index", Index: &IndexDef{Name: "ix_ed_titulo", Columns: []string{"titulo", "edad"}, Unique: true}})
	st, _ = desc.Describe(ref("ed"))
	if len(st.Indexes) != 1 || !st.Indexes[0].Unique || len(st.Indexes[0].Columns) != 2 {
		t.Errorf("índices: %+v", st.Indexes)
	}
	apply("ed", SchemaOp{Kind: "drop_index", Target: "ix_ed_titulo"})
	apply("ed", SchemaOp{Kind: "drop_column", Target: "edad"})
	if got := colNames("ed"); len(got) != 2 || got[1] != "titulo" {
		t.Errorf("columnas tras renombrar y borrar: %v", got)
	}
	apply("ed", SchemaOp{Kind: "rename_table", NewName: "ed2"})
	if _, err := desc.Describe(ref("ed")); err == nil {
		t.Error("la tabla vieja no debería existir")
	}
	if got := colNames("ed2"); len(got) != 2 {
		t.Errorf("ed2: %v", got)
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM ed2`).Scan(&n)
	if n != 2 {
		t.Errorf("los datos deben sobrevivir al renombrado: %d filas", n)
	}
	apply("ed2", SchemaOp{Kind: "drop_table"})
	if _, err := desc.Describe(ref("ed2")); err == nil {
		t.Error("la tabla borrada no debería existir")
	}

	// Lo que SQLite no soporta debe fallar con un mensaje claro y sin tocar nada.
	if _, err := edit.Apply(ref("items"), SchemaOp{Kind: "alter_column", Target: "nota", Column: &ColumnDef{Type: "INT", Nullable: true}}); err == nil || !strings.Contains(err.Error(), "recrear") {
		t.Errorf("alter_column en SQLite: %v", err)
	}
	// Las vistas solo admiten renombrar y borrar.
	db.Exec(`CREATE VIEW vv AS SELECT id FROM items`)
	if _, err := edit.Preview(ref("vv"), SchemaOp{Kind: "add_column", Column: &ColumnDef{Name: "x", Type: "INT", Nullable: true}}); err == nil {
		t.Error("no se puede añadir una columna a una vista")
	}
	if p, err := edit.Preview(ref("vv"), SchemaOp{Kind: "drop_table"}); err != nil || !strings.HasPrefix(p.Statements[0], "DROP VIEW") {
		t.Errorf("borrar vista: %v %+v", err, p)
	}
	// Preview no ejecuta nada.
	edit.Preview(ref("items"), SchemaOp{Kind: "drop_table"})
	if _, err := desc.Describe(ref("items")); err != nil {
		t.Error("Preview no debe ejecutar la operación")
	}
}
