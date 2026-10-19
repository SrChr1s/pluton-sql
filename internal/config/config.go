// Package config persiste los perfiles de conexión (sin contraseñas).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// ConnectionProfile describe cómo conectarse a una base de datos.
type ConnectionProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Driver   string `json:"driver"` // postgres | mysql | sqlite | mssql
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
	FilePath string `json:"filePath"` // solo SQLite
	SSLMode  string `json:"sslMode"`
	// TLSMinVersion permite negociar TLS antiguo con servidores legacy: "", "1.0", "1.1" o "1.2".
	TLSMinVersion string `json:"tlsMinVersion"`
	// Color es la etiqueta de color de la conexión: "" (sin color) o "#rrggbb".
	Color string `json:"color"`
	// Workspace es el id del workspace al que pertenece ("" = ninguno).
	Workspace string `json:"workspace"`
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Store guarda los perfiles en un archivo JSON.
type Store struct {
	mu   sync.Mutex
	path string
}

// Dir devuelve el directorio de configuración de la aplicación (os.UserConfigDir()/pluton-sql).
func Dir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pluton-sql"), nil
}

// NewStore crea un Store en path. Si path está vacío usa os.UserConfigDir()/pluton-sql/connections.json.
func NewStore(path string) (*Store, error) {
	if path == "" {
		dir, err := Dir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "connections.json")
	}
	return &Store{path: path}, nil
}

func (s *Store) load() ([]ConnectionProfile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ps []ConnectionProfile
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("config corrupta %s: %w", s.path, err)
	}
	return ps, nil
}

func (s *Store) write(ps []ConnectionProfile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// List devuelve los perfiles ordenados por nombre.
func (s *Store) List() ([]ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	return ps, nil
}

// Get busca un perfil por ID.
func (s *Store) Get(id string) (ConnectionProfile, error) {
	ps, err := s.List()
	if err != nil {
		return ConnectionProfile{}, err
	}
	for _, p := range ps {
		if p.ID == id {
			return p, nil
		}
	}
	return ConnectionProfile{}, fmt.Errorf("conexión %q no encontrada", id)
}

// Save crea (ID vacío) o actualiza un perfil y devuelve el resultado.
func (s *Store) Save(p ConnectionProfile) (ConnectionProfile, error) {
	if p.Color != "" && !colorRe.MatchString(p.Color) {
		return p, fmt.Errorf("color %q no válido: se espera #rrggbb", p.Color)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.load()
	if err != nil {
		return p, err
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
		ps = append(ps, p)
	} else {
		found := false
		for i := range ps {
			if ps[i].ID == p.ID {
				ps[i], found = p, true
				break
			}
		}
		if !found {
			ps = append(ps, p)
		}
	}
	return p, s.write(ps)
}

// Delete elimina un perfil.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.load()
	if err != nil {
		return err
	}
	out := ps[:0]
	for _, p := range ps {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return s.write(out)
}
