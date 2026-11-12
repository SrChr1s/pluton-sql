package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/secrets"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

func newConnService(t *testing.T) *ConnectionService {
	t.Helper()
	keyring.MockInit()
	store, err := config.NewStore(filepath.Join(t.TempDir(), "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	return NewConnectionService(store, session.NewManager(store), nil)
}

func TestSaveClone(t *testing.T) {
	s := newConnService(t)
	orig, err := s.Save(config.ConnectionProfile{Name: "Prod", Driver: "postgres", Host: "h", Port: 5432, User: "u", Database: "d", Color: "#ff6b6b"}, "secreto")
	if err != nil {
		t.Fatal(err)
	}

	// Sin contraseña nueva hereda la de la original; el ID que traiga el perfil se ignora.
	clone := orig
	clone.Name = "Prod (copia)"
	c1, err := s.SaveClone(orig.ID, clone, "")
	if err != nil {
		t.Fatal(err)
	}
	if c1.ID == "" || c1.ID == orig.ID {
		t.Fatalf("el clon debe tener un ID nuevo: %q (original %q)", c1.ID, orig.ID)
	}
	if pw, _ := secrets.Get(c1.ID); pw != "secreto" {
		t.Errorf("contraseña del clon = %q, se esperaba la de la original", pw)
	}

	// Con contraseña nueva usa esa y no toca la original.
	c2, err := s.SaveClone(orig.ID, clone, "otra")
	if err != nil {
		t.Fatal(err)
	}
	if pw, _ := secrets.Get(c2.ID); pw != "otra" {
		t.Errorf("contraseña del clon 2 = %q", pw)
	}
	if pw, _ := secrets.Get(orig.ID); pw != "secreto" {
		t.Errorf("la original cambió: %q", pw)
	}

	ps, _ := s.List()
	if len(ps) != 3 {
		t.Fatalf("conexiones = %d, se esperaban 3", len(ps))
	}
	for _, p := range ps {
		if p.ID == orig.ID && p.Name != "Prod" {
			t.Errorf("la original se modificó: %+v", p)
		}
		if p.ID == c1.ID && (p.Name != "Prod (copia)" || p.Host != "h" || p.Color != "#ff6b6b") {
			t.Errorf("el clon no copió los datos: %+v", p)
		}
	}
}

// Test es lo que decide si "Guardar y conectar" guarda: debe fallar con datos malos y pasar con datos buenos.
func TestTestConnection(t *testing.T) {
	s := newConnService(t)
	bad := config.ConnectionProfile{Name: "x", Driver: "postgres", Host: "127.0.0.1", Port: 1, User: "u", Database: "d"}
	if err := s.Test(bad, "pw"); err == nil {
		t.Error("un servidor inalcanzable debe dar error")
	}
	path := filepath.Join(t.TempDir(), "ok.db")
	good := config.ConnectionProfile{Name: "lite", Driver: "sqlite", FilePath: path}
	if err := s.Test(good, ""); err == nil {
		t.Error("un archivo SQLite que no existe debe dar error")
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Test(good, ""); err != nil {
		t.Errorf("una base SQLite válida debe conectar: %v", err)
	}
}
