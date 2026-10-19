package service

import (
	"path/filepath"
	"testing"

	"github.com/SrChr1s/pluton-sql/internal/config"
)

func newWS(t *testing.T) (*WorkspaceService, *config.Store) {
	t.Helper()
	dir := t.TempDir()
	conns, _ := config.NewStore(filepath.Join(dir, "c.json"))
	wss, _ := config.NewWorkspaceStore(filepath.Join(dir, "w.json"))
	return NewWorkspaceService(wss, conns), conns
}

func TestWorkspaceValidation(t *testing.T) {
	s, _ := newWS(t)
	if _, err := s.Save(config.Workspace{Name: "  "}); err == nil {
		t.Error("el nombre vacío debería rechazarse")
	}
	a, err := s.Save(config.Workspace{Name: "Producción"})
	if err != nil || a.ID == "" {
		t.Fatalf("save: %v", err)
	}
	if _, err := s.Save(config.Workspace{Name: "producción"}); err == nil {
		t.Error("un nombre repetido (sin distinguir mayúsculas) debería rechazarse")
	}
	a.Name = "Prod"
	if _, err := s.Save(a); err != nil {
		t.Fatalf("renombrar: %v", err)
	}
	if _, err := s.Save(config.Workspace{ID: "nope", Name: "x"}); err == nil {
		t.Error("renombrar un workspace inexistente debería fallar")
	}
	long := make([]rune, config.MaxWorkspaceName+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := s.Save(config.Workspace{Name: string(long)}); err == nil {
		t.Error("un nombre demasiado largo debería rechazarse")
	}
	if ws, _ := s.List(); len(ws) != 1 || ws[0].Name != "Prod" {
		t.Errorf("lista inesperada: %+v", ws)
	}
}

func TestWorkspaceDeleteKeepsConnections(t *testing.T) {
	s, conns := newWS(t)
	w, _ := s.Save(config.Workspace{Name: "Cliente X"})
	in, _ := conns.Save(config.ConnectionProfile{Name: "a", Driver: "sqlite", Workspace: w.ID})
	out, _ := conns.Save(config.ConnectionProfile{Name: "b", Driver: "sqlite"})

	if err := s.Delete(w.ID); err != nil {
		t.Fatal(err)
	}
	if ws, _ := s.List(); len(ws) != 0 {
		t.Errorf("el workspace debería haberse borrado: %+v", ws)
	}
	got, err := conns.Get(in.ID)
	if err != nil || got.Workspace != "" {
		t.Errorf("la conexión debería seguir existiendo y quedar sin workspace: %+v %v", got, err)
	}
	if _, err := conns.Get(out.ID); err != nil {
		t.Errorf("la otra conexión no debería tocarse: %v", err)
	}
}

func TestMoveConnection(t *testing.T) {
	s, conns := newWS(t)
	w, _ := s.Save(config.Workspace{Name: "Local"})
	p, _ := conns.Save(config.ConnectionProfile{Name: "a", Driver: "postgres", Host: "h", Port: 5432, Color: "#ff0000"})

	got, err := s.MoveConnection(p.ID, w.ID)
	if err != nil || got.Workspace != w.ID {
		t.Fatalf("mover: %+v %v", got, err)
	}
	if got.Host != "h" || got.Port != 5432 || got.Color != "#ff0000" {
		t.Errorf("mover no debe alterar el resto de campos: %+v", got)
	}
	if again, _ := conns.Get(p.ID); again.Workspace != w.ID {
		t.Errorf("no se guardó: %+v", again)
	}
	if got, err := s.MoveConnection(p.ID, ""); err != nil || got.Workspace != "" {
		t.Errorf("sacar del workspace: %+v %v", got, err)
	}
	if _, err := s.MoveConnection(p.ID, "no-existe"); err == nil {
		t.Error("mover a un workspace inexistente debería fallar")
	}
	if _, err := s.MoveConnection("no-existe", w.ID); err == nil {
		t.Error("mover una conexión inexistente debería fallar")
	}
}
