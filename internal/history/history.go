// Package history guarda el historial de consultas ejecutadas y las consultas guardadas por el usuario.
//
// Los archivos son JSON en el directorio de configuración, con permisos 0600: el SQL puede contener
// datos sensibles (p. ej. un INSERT con contraseñas), así que no debe ser legible por otros usuarios.
package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxSQLBytes limita el SQL guardado por entrada (un volcado enorme no debe inflar el archivo).
	MaxSQLBytes = 32 * 1024
	// MaxPerProfile y MaxTotal acotan el tamaño del historial; se descartan las entradas más antiguas.
	MaxPerProfile = 500
	MaxTotal      = 3000
)

// Entry es una ejecución registrada.
type Entry struct {
	ID         string    `json:"id"`
	Profile    string    `json:"profile"`
	Database   string    `json:"database"`
	SQL        string    `json:"sql"`
	At         time.Time `json:"at"`
	DurationMs int64     `json:"durationMs"`
	Rows       int64     `json:"rows"` // filas devueltas o afectadas
	IsQuery    bool      `json:"isQuery"`
	Error      string    `json:"error"`
	// Count cuenta las ejecuciones consecutivas idénticas, que se agrupan en una sola entrada.
	Count int `json:"count"`
}

// writeJSON escribe de forma atómica (archivo temporal + rename) con permisos restrictivos.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

// Store es el historial de ejecuciones.
type Store struct {
	mu      sync.Mutex
	path    string
	entries []Entry // más antigua primero
	loaded  bool
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) load() error {
	if s.loaded {
		return nil
	}
	if err := readJSON(s.path, &s.entries); err != nil {
		return err
	}
	s.loaded = true
	return nil
}

func truncate(sql string) string {
	if len(sql) <= MaxSQLBytes {
		return sql
	}
	cut := MaxSQLBytes
	for cut > 0 && sql[cut]&0xC0 == 0x80 { // no partir un carácter UTF-8 por la mitad
		cut--
	}
	return sql[:cut] + "\n-- … [truncado]"
}

// Add registra una ejecución. Si es idéntica a la anterior del mismo perfil, se agrupa en ella.
func (s *Store) Add(e Entry) error {
	e.SQL = truncate(strings.TrimSpace(e.SQL))
	if e.SQL == "" || e.Profile == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	for i := len(s.entries) - 1; i >= 0; i-- {
		last := s.entries[i]
		if last.Profile != e.Profile {
			continue
		}
		if last.SQL == e.SQL && last.Database == e.Database {
			e.ID, e.Count = last.ID, last.Count+1
			s.entries = append(s.entries[:i], s.entries[i+1:]...) // sube al final, como la más reciente
			s.entries = append(s.entries, e)
			return s.save()
		}
		break // solo se agrupan ejecuciones consecutivas
	}
	e.ID, e.Count = uuid.NewString(), 1
	s.entries = append(s.entries, e)
	s.trim(e.Profile)
	return s.save()
}

// trim descarta las entradas más antiguas que excedan los límites.
func (s *Store) trim(profile string) {
	n := 0
	for _, x := range s.entries {
		if x.Profile == profile {
			n++
		}
	}
	for i := 0; n > MaxPerProfile && i < len(s.entries); {
		if s.entries[i].Profile == profile {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			n--
			continue
		}
		i++
	}
	if len(s.entries) > MaxTotal {
		s.entries = s.entries[len(s.entries)-MaxTotal:]
	}
}

func (s *Store) save() error { return writeJSON(s.path, s.entries) }

// List devuelve las entradas del perfil, la más reciente primero, filtradas por texto (sin distinguir mayúsculas).
func (s *Store) List(profile, search string, limit int) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(search))
	out := []Entry{}
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.Profile != profile || (q != "" && !strings.Contains(strings.ToLower(e.SQL), q)) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Delete borra una entrada.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			return s.save()
		}
	}
	return nil
}

// Clear borra todo el historial de un perfil.
func (s *Store) Clear(profile string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	kept := s.entries[:0]
	for _, e := range s.entries {
		if e.Profile != profile {
			kept = append(kept, e)
		}
	}
	s.entries = kept
	return s.save()
}

// ---- consultas guardadas ----

// Saved es una consulta con nombre, global (sirve para cualquier conexión).
type Saved struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SQL       string    `json:"sql"`
	Driver    string    `json:"driver"` // motor para el que se escribió (orientativo)
	UpdatedAt time.Time `json:"updatedAt"`
}

type SavedStore struct {
	mu     sync.Mutex
	path   string
	items  []Saved
	loaded bool
}

func NewSavedStore(path string) *SavedStore { return &SavedStore{path: path} }

func (s *SavedStore) load() error {
	if s.loaded {
		return nil
	}
	if err := readJSON(s.path, &s.items); err != nil {
		return err
	}
	s.loaded = true
	return nil
}

// Save crea (id vacío) o actualiza una consulta guardada. El nombre no puede repetirse.
func (s *SavedStore) Save(q Saved) (Saved, error) {
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" {
		return q, errors.New("la consulta necesita un nombre")
	}
	if strings.TrimSpace(q.SQL) == "" {
		return q, errors.New("no hay SQL que guardar")
	}
	q.SQL = truncate(q.SQL)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return q, err
	}
	for _, x := range s.items {
		if x.ID != q.ID && strings.EqualFold(x.Name, q.Name) {
			return q, errors.New("ya existe una consulta guardada con ese nombre")
		}
	}
	q.UpdatedAt = time.Now()
	if q.ID == "" {
		q.ID = uuid.NewString()
		s.items = append(s.items, q)
	} else {
		found := false
		for i := range s.items {
			if s.items[i].ID == q.ID {
				s.items[i], found = q, true
			}
		}
		if !found {
			s.items = append(s.items, q)
		}
	}
	return q, writeJSON(s.path, s.items)
}

// List devuelve las consultas ordenadas por nombre.
func (s *SavedStore) List() ([]Saved, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	out := append([]Saved{}, s.items...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (s *SavedStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	for i, x := range s.items {
		if x.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return writeJSON(s.path, s.items)
		}
	}
	return nil
}
