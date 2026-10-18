package service

import (
	"context"

	"github.com/SrChr1s/pluton-sql/internal/driver"
)

// TableStructure es todo lo que se muestra en la vista de estructura de una tabla o vista.
type TableStructure struct {
	Columns     []driver.ColumnInfo     `json:"columns"`
	Indexes     []driver.IndexInfo      `json:"indexes"`
	ForeignKeys []driver.ForeignKeyInfo `json:"foreignKeys"`
	DDL         string                  `json:"ddl"`
	// DDLError se rellena si no se pudo reconstruir el DDL; el resto de la estructura sigue siendo válido.
	DDLError string `json:"ddlError"`
	IsView   bool   `json:"isView"`
}

type StructureService struct{ pools Pools }

func NewStructureService(pools Pools) *StructureService { return &StructureService{pools: pools} }

// Describe devuelve columnas, índices, claves foráneas y el DDL de la tabla.
func (s *StructureService) Describe(ref TableRef) (*TableStructure, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	t, db, d, cols, err := openTable(ctx, s.pools, ref)
	if err != nil {
		return nil, err
	}
	out := &TableStructure{Columns: cols, Indexes: []driver.IndexInfo{}, ForeignKeys: []driver.ForeignKeyInfo{}, IsView: t.Type == "view"}

	if !out.IsView { // las vistas no tienen índices ni claves foráneas
		if out.Indexes, err = d.ListIndexes(ctx, db, ref.Schema, ref.Table); err != nil {
			return nil, err
		}
		if out.ForeignKeys, err = d.ListForeignKeys(ctx, db, ref.Schema, ref.Table); err != nil {
			return nil, err
		}
	}
	if out.Indexes == nil {
		out.Indexes = []driver.IndexInfo{}
	}
	if out.ForeignKeys == nil {
		out.ForeignKeys = []driver.ForeignKeyInfo{}
	}

	if out.DDL, err = d.TableDDL(ctx, db, ref.Schema, ref.Table, out.IsView); err != nil {
		out.DDL, out.DDLError = "", err.Error()
	}
	return out, nil
}
