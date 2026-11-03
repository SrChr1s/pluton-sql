package service

import (
	"context"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

type SchemaService struct{ sess *session.Manager }

func NewSchemaService(sess *session.Manager) *SchemaService { return &SchemaService{sess: sess} }

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// Databases lista las bases de datos del servidor del perfil.
func (s *SchemaService) Databases(profileID string) ([]string, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, "")
	if err != nil {
		return nil, err
	}
	return d.ListDatabases(c, db)
}

// Schemas lista los esquemas de una base ([""] si el motor no los tiene).
func (s *SchemaService) Schemas(profileID, database string) ([]string, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, database)
	if err != nil {
		return nil, err
	}
	return d.ListSchemas(c, db)
}

func (s *SchemaService) Tables(profileID, database, schema string) ([]driver.TableInfo, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, database)
	if err != nil {
		return nil, err
	}
	return d.ListTables(c, db, schema)
}

func (s *SchemaService) Columns(profileID, database, schema, table string) ([]driver.ColumnInfo, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, database)
	if err != nil {
		return nil, err
	}
	return d.ListColumns(c, db, schema, table)
}

func (s *SchemaService) Indexes(profileID, database, schema, table string) ([]driver.IndexInfo, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, database)
	if err != nil {
		return nil, err
	}
	return d.ListIndexes(c, db, schema, table)
}

// Routines lista los procedimientos y funciones de un esquema.
func (s *SchemaService) Routines(profileID, database, schema string) ([]driver.RoutineInfo, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, database)
	if err != nil {
		return nil, err
	}
	return d.ListRoutines(c, db, schema)
}

// RoutineDefinition devuelve el SQL de creación de una rutina.
func (s *SchemaService) RoutineDefinition(profileID, database string, r driver.RoutineInfo) (string, error) {
	c, cancel := ctx()
	defer cancel()
	db, d, err := s.sess.Get(c, profileID, database)
	if err != nil {
		return "", err
	}
	return d.RoutineDDL(c, db, r.Schema, r)
}
