package service

import "github.com/SrChr1s/pluton-sql/internal/config"

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
