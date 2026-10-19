package service

import (
	"errors"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

// WorkspaceService gestiona los workspaces que agrupan las conexiones guardadas.
type WorkspaceService struct {
	workspaces *config.WorkspaceStore
	conns      *config.Store
}

func NewWorkspaceService(workspaces *config.WorkspaceStore, conns *config.Store) *WorkspaceService {
	return &WorkspaceService{workspaces: workspaces, conns: conns}
}

func (s *WorkspaceService) List() ([]config.Workspace, error) { return s.workspaces.List() }

// Save crea (id vacío) o renombra un workspace.
func (s *WorkspaceService) Save(w config.Workspace) (config.Workspace, error) {
	return s.workspaces.Save(w)
}

// MoveConnection asigna una conexión a un workspace ("" = ninguno). Solo cambia esa pertenencia:
// no cierra los pools ni toca la contraseña, así que se puede mover una conexión en uso.
func (s *WorkspaceService) MoveConnection(connectionID, workspaceID string) (config.ConnectionProfile, error) {
	if workspaceID != "" {
		ws, err := s.workspaces.List()
		if err != nil {
			return config.ConnectionProfile{}, err
		}
		found := false
		for _, w := range ws {
			if w.ID == workspaceID {
				found = true
				break
			}
		}
		if !found {
			return config.ConnectionProfile{}, errors.New("el workspace no existe")
		}
	}
	p, err := s.conns.Get(connectionID)
	if err != nil {
		return p, err
	}
	p.Workspace = workspaceID
	return s.conns.Save(p)
}

// Delete elimina el workspace; sus conexiones no se borran, quedan sin workspace.
func (s *WorkspaceService) Delete(id string) error {
	ps, err := s.conns.List()
	if err != nil {
		return err
	}
	for _, p := range ps {
		if p.Workspace == id {
			p.Workspace = ""
			if _, err := s.conns.Save(p); err != nil {
				return err
			}
		}
	}
	return s.workspaces.Delete(id)
}
