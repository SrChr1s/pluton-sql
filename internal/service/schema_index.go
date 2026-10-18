package service

import (
	"context"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/driver"
)

// IndexColumn es una columna para el autocompletado.
type IndexColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// IndexTable es una tabla o vista con sus columnas.
type IndexTable struct {
	Schema  string        `json:"schema"` // "" si el motor no tiene esquemas
	Name    string        `json:"name"`
	IsView  bool          `json:"isView"`
	Columns []IndexColumn `json:"columns"`
}

// SchemaIndex es el catálogo completo de una base de datos, pensado para el autocompletado del editor.
type SchemaIndex struct {
	Tables []IndexTable `json:"tables"`
	// DefaultSchema es el esquema cuyas tablas se pueden escribir sin cualificar ("public", "dbo" o "").
	DefaultSchema string `json:"defaultSchema"`
}

// defaultSchemas son los esquemas por defecto de cada motor.
var defaultSchemas = map[string]string{"postgres": "public", "mssql": "dbo"}

// Index devuelve todas las tablas, vistas y columnas de la base de datos en una sola consulta.
func (s *SchemaService) Index(profileID, database string) (*SchemaIndex, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, d, err := s.sess.Get(ctx, profileID, database)
	if err != nil {
		return nil, err
	}
	refs, err := d.ListAllColumns(ctx, db)
	if err != nil {
		return nil, err
	}
	return groupIndex(d.Name(), refs), nil
}

// groupIndex agrupa las filas (ordenadas por esquema y tabla) en tablas con sus columnas.
func groupIndex(engine string, refs []driver.ColumnRef) *SchemaIndex {
	idx := &SchemaIndex{Tables: []IndexTable{}, DefaultSchema: defaultSchemas[engine]}
	for _, r := range refs {
		n := len(idx.Tables)
		if n == 0 || idx.Tables[n-1].Schema != r.Schema || idx.Tables[n-1].Name != r.Table {
			idx.Tables = append(idx.Tables, IndexTable{Schema: r.Schema, Name: r.Table, IsView: r.IsView, Columns: []IndexColumn{}})
			n++
		}
		idx.Tables[n-1].Columns = append(idx.Tables[n-1].Columns, IndexColumn{Name: r.Column, Type: r.DataType})
	}
	return idx
}
