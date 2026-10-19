// Package session mantiene los pools *sql.DB abiertos por perfil y base de datos.
package session

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/secrets"
)

type entry struct {
	db      *sql.DB
	dialect driver.Dialect
}

// Manager abre y reutiliza pools. Es seguro para uso concurrente.
type Manager struct {
	store *config.Store
	mu    sync.Mutex
	pools map[string]entry
}

func NewManager(store *config.Store) *Manager {
	return &Manager{store: store, pools: map[string]entry{}}
}

func key(profileID, database string) string { return profileID + "\x00" + database }

// Get devuelve el pool del perfil para database (vacío = la del perfil).
func (m *Manager) Get(ctx context.Context, profileID, database string) (*sql.DB, driver.Dialect, error) {
	k := key(profileID, database)
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.pools[k]; ok {
		return e.db, e.dialect, nil
	}
	p, err := m.store.Get(profileID)
	if err != nil {
		return nil, nil, err
	}
	pw, err := secrets.Get(profileID)
	if err != nil {
		return nil, nil, fmt.Errorf("leyendo contraseña del llavero: %w", err)
	}
	db, d, err := Open(ctx, p, pw, database)
	if err != nil {
		return nil, nil, err
	}
	m.pools[k] = entry{db, d}
	return db, d, nil
}

// Open abre un pool nuevo y verifica la conexión con Ping.
func Open(ctx context.Context, p config.ConnectionProfile, password, database string) (*sql.DB, driver.Dialect, error) {
	d, err := driver.Get(p.Driver)
	if err != nil {
		return nil, nil, err
	}
	var db *sql.DB
	if o, ok := d.(driver.Opener); ok {
		db, err = o.Open(p, password, database)
	} else {
		db, err = sql.Open(d.DriverName(), d.DSN(p, password, database))
	}
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pctx); err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, d, nil
}

// Close cierra todos los pools de un perfil (p. ej. tras editarlo o borrarlo).
func (m *Manager) Close(profileID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := profileID + "\x00"
	for k, e := range m.pools {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			e.db.Close()
			delete(m.pools, k)
		}
	}
}

// CloseDatabase cierra solo el pool de una base de datos del perfil. Si es la base por
// defecto del perfil, cierra también el pool sin nombre (que apunta a ella).
func (m *Manager) CloseDatabase(profileID, database string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := []string{key(profileID, database)}
	if p, err := m.store.Get(profileID); err == nil && database != "" && database == p.Database {
		keys = append(keys, key(profileID, ""))
	}
	for _, k := range keys {
		if e, ok := m.pools[k]; ok {
			e.db.Close()
			delete(m.pools, k)
		}
	}
}

// CloseAll cierra todos los pools.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.pools {
		e.db.Close()
		delete(m.pools, k)
	}
}
