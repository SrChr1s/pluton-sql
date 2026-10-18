// Package service contiene los structs que Wails expone al frontend.
package service

import (
	"context"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/secrets"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

type ConnectionService struct {
	store *config.Store
	sess  *session.Manager
	// onDelete limpia los datos asociados a una conexión (p. ej. su historial) al eliminarla.
	onDelete func(profileID string)
}

func NewConnectionService(store *config.Store, sess *session.Manager, onDelete func(profileID string)) *ConnectionService {
	return &ConnectionService{store: store, sess: sess, onDelete: onDelete}
}

// Drivers lista los motores soportados.
func (s *ConnectionService) Drivers() []string { return driver.Names() }

// DefaultPort devuelve el puerto por defecto del motor.
func (s *ConnectionService) DefaultPort(name string) int {
	d, err := driver.Get(name)
	if err != nil {
		return 0
	}
	return d.DefaultPort()
}

func (s *ConnectionService) List() ([]config.ConnectionProfile, error) { return s.store.List() }

// Save guarda el perfil. Si password no está vacía se actualiza en el llavero;
// vacía conserva la existente.
func (s *ConnectionService) Save(p config.ConnectionProfile, password string) (config.ConnectionProfile, error) {
	p, err := s.store.Save(p)
	if err != nil {
		return p, err
	}
	if password != "" {
		if err := secrets.Set(p.ID, password); err != nil {
			return p, err
		}
	}
	s.sess.Close(p.ID) // fuerza reconexión con los datos nuevos
	return p, nil
}

// Disconnect cierra los pools abiertos del perfil. La siguiente operación
// vuelve a conectar.
func (s *ConnectionService) Disconnect(id string) {
	s.sess.Close(id)
}

func (s *ConnectionService) Delete(id string) error {
	s.sess.Close(id)
	if err := secrets.Delete(id); err != nil {
		return err
	}
	if err := s.store.Delete(id); err != nil {
		return err
	}
	if s.onDelete != nil {
		s.onDelete(id)
	}
	return nil
}

// Test prueba una conexión sin guardarla. Si password está vacía y el perfil
// ya existe, usa la del llavero.
func (s *ConnectionService) Test(p config.ConnectionProfile, password string) error {
	if password == "" && p.ID != "" {
		pw, err := secrets.Get(p.ID)
		if err != nil {
			return err
		}
		password = pw
	}
	db, _, err := session.Open(context.Background(), p, password, "")
	if err != nil {
		return err
	}
	return db.Close()
}
