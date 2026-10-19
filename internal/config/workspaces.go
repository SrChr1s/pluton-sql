package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// MaxWorkspaceName limita la longitud del nombre de un workspace.
const MaxWorkspaceName = 60

// Workspace agrupa conexiones guardadas bajo un nombre.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkspaceStore guarda los workspaces en un archivo JSON.
type WorkspaceStore struct {
	mu   sync.Mutex
	path string
}

// NewWorkspaceStore crea un store en path. Si está vacío usa workspaces.json junto a connections.json.
func NewWorkspaceStore(path string) (*WorkspaceStore, error) {
	if path == "" {
		dir, err := Dir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "workspaces.json")
	}
	return &WorkspaceStore{path: path}, nil
}

func (s *WorkspaceStore) load() ([]Workspace, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ws []Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("config corrupta %s: %w", s.path, err)
	}
	return ws, nil
}

func (s *WorkspaceStore) write(ws []Workspace) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// List devuelve los workspaces en el orden en que se crearon.
func (s *WorkspaceStore) List() ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Save crea (id vacío) o renombra un workspace. El nombre no puede estar vacío ni repetirse.
func (s *WorkspaceStore) Save(w Workspace) (Workspace, error) {
	w.Name = strings.TrimSpace(w.Name)
	if w.Name == "" {
		return w, errors.New("el nombre del workspace es obligatorio")
	}
	if len([]rune(w.Name)) > MaxWorkspaceName {
		return w, fmt.Errorf("el nombre del workspace admite hasta %d caracteres", MaxWorkspaceName)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, err := s.load()
	if err != nil {
		return w, err
	}
	for _, o := range ws {
		if o.ID != w.ID && strings.EqualFold(o.Name, w.Name) {
			return w, fmt.Errorf("ya existe un workspace llamado %q", o.Name)
		}
	}
	if w.ID == "" {
		w.ID = uuid.NewString()
		ws = append(ws, w)
	} else {
		found := false
		for i := range ws {
			if ws[i].ID == w.ID {
				ws[i], found = w, true
				break
			}
		}
		if !found {
			return w, errors.New("el workspace no existe")
		}
	}
	return w, s.write(ws)
}

// Delete elimina un workspace. No toca las conexiones: quien lo llama las reasigna.
func (s *WorkspaceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, err := s.load()
	if err != nil {
		return err
	}
	out := ws[:0]
	for _, w := range ws {
		if w.ID != id {
			out = append(out, w)
		}
	}
	return s.write(out)
}
