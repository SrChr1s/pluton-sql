package service

import (
	"context"
	"database/sql"
	"fmt"
)

// SchemaEditService modifica la estructura (columnas, índices, claves foráneas, tablas).
// Funciona en dos pasos: Preview devuelve el SQL para revisarlo y Apply lo ejecuta. Apply vuelve a
// generar el SQL en el servidor: nunca se ejecuta texto que llegue de la interfaz.
type SchemaEditService struct{ pools Pools }

func NewSchemaEditService(pools Pools) *SchemaEditService { return &SchemaEditService{pools: pools} }

func (s *SchemaEditService) plan(ctx context.Context, ref TableRef, op SchemaOp) (*SchemaPlan, *sql.DB, error) {
	if op.Kind == "create_table" {
		db, d, err := s.pools.Get(ctx, ref.Profile, ref.Database)
		if err != nil {
			return nil, nil, err
		}
		p, err := buildPlan(d, ref, op, nil, false)
		return p, db, err
	}
	t, db, d, cols, err := openTable(ctx, s.pools, ref)
	if err != nil {
		return nil, nil, err
	}
	isView := t.Type == "view"
	if isView && op.Kind != "rename_table" && op.Kind != "drop_table" {
		return nil, nil, fmt.Errorf("las vistas solo se pueden renombrar o eliminar")
	}
	p, err := buildPlan(d, ref, op, cols, isView)
	return p, db, err
}

// Preview devuelve el SQL que ejecutaría la operación, sin ejecutar nada.
func (s *SchemaEditService) Preview(ref TableRef, op SchemaOp) (*SchemaPlan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	p, _, err := s.plan(ctx, ref, op)
	return p, err
}

// Apply ejecuta la operación. En PostgreSQL, SQLite y SQL Server todas las sentencias van en una
// transacción; MySQL confirma cada sentencia de estructura por separado (lo avisa el plan).
func (s *SchemaEditService) Apply(ref TableRef, op SchemaOp) (*SchemaPlan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	p, db, err := s.plan(ctx, ref, op)
	if err != nil {
		return nil, err
	}
	if !p.Transactional {
		for i, st := range p.Statements {
			if _, err := db.ExecContext(ctx, st); err != nil {
				return nil, fmt.Errorf("sentencia %d de %d: %w", i+1, len(p.Statements), err)
			}
		}
		return p, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // no-op tras Commit
	for i, st := range p.Statements {
		if _, err := tx.ExecContext(ctx, st); err != nil {
			return nil, fmt.Errorf("sentencia %d de %d: %w (no se aplicó ningún cambio)", i+1, len(p.Statements), err)
		}
	}
	return p, tx.Commit()
}
