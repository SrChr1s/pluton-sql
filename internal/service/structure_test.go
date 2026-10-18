package service

import (
	"strings"
	"testing"
)

func TestDescribeSQLite(t *testing.T) {
	_, db := newData(t)
	for _, q := range []string{
		`CREATE TABLE padre (id INTEGER PRIMARY KEY, codigo TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX ux_padre_codigo ON padre (codigo)`,
		`CREATE TABLE hijo (
			id INTEGER PRIMARY KEY, padre_id INT NOT NULL, otro_a INT, otro_b INT,
			FOREIGN KEY (padre_id) REFERENCES padre (id) ON DELETE CASCADE ON UPDATE SET NULL,
			FOREIGN KEY (otro_a, otro_b) REFERENCES padre (id, id))`,
		`CREATE INDEX ix_hijo_padre_otro ON hijo (padre_id, otro_a)`,
		`CREATE VIEW v_hijo AS SELECT id FROM hijo`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	svc := NewStructureService(fakePools{db, mustDialect(t)})

	st, err := svc.Describe(ref("hijo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Columns) != 4 || !st.Columns[0].PrimaryKey || !st.Columns[0].AutoIncrement || st.Columns[1].AutoIncrement {
		t.Errorf("columnas: %+v", st.Columns)
	}
	if len(st.Indexes) != 1 || st.Indexes[0].Name != "ix_hijo_padre_otro" || len(st.Indexes[0].Columns) != 2 || st.Indexes[0].Unique {
		t.Errorf("índices: %+v", st.Indexes)
	}
	if len(st.ForeignKeys) != 2 {
		t.Fatalf("claves foráneas: %+v", st.ForeignKeys)
	}
	var simple, compuesta = st.ForeignKeys[0], st.ForeignKeys[1]
	if len(simple.Columns) == 2 { // SQLite las numera en orden inverso al de declaración
		simple, compuesta = compuesta, simple
	}
	if simple.RefTable != "padre" || simple.Columns[0] != "padre_id" || simple.RefColumns[0] != "id" || simple.OnDelete != "CASCADE" || simple.OnUpdate != "SET NULL" {
		t.Errorf("fk simple: %+v", simple)
	}
	if len(compuesta.Columns) != 2 || len(compuesta.RefColumns) != 2 || compuesta.Columns[1] != "otro_b" {
		t.Errorf("fk compuesta: %+v", compuesta)
	}
	if !strings.Contains(st.DDL, "CREATE TABLE") || !strings.Contains(st.DDL, "CREATE INDEX ix_hijo_padre_otro") || st.DDLError != "" || st.IsView {
		t.Errorf("DDL: %q err=%q", st.DDL, st.DDLError)
	}

	// Índice único y marca de clave primaria en otra tabla.
	p, _ := svc.Describe(ref("padre"))
	var uniq bool
	for _, ix := range p.Indexes {
		uniq = uniq || (ix.Name == "ux_padre_codigo" && ix.Unique && !ix.Primary)
	}
	if !uniq {
		t.Errorf("índice único: %+v", p.Indexes)
	}

	// Vistas: sin índices ni claves, y con DDL.
	v, err := svc.Describe(ref("v_hijo"))
	if err != nil || !v.IsView || len(v.Indexes) != 0 || len(v.ForeignKeys) != 0 || !strings.Contains(v.DDL, "CREATE VIEW") {
		t.Errorf("vista: %v %+v", err, v)
	}
	if v.Indexes == nil || v.ForeignKeys == nil {
		t.Error("las listas vacías deben ser [] y no null para el frontend")
	}
	if _, err := svc.Describe(ref("no_existe")); err == nil {
		t.Error("una tabla inexistente debe dar error")
	}
}
