package config

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundtrip(t *testing.T) {
	s, _ := NewStore(filepath.Join(t.TempDir(), "c.json"))
	if ps, err := s.List(); err != nil || len(ps) != 0 {
		t.Fatalf("esperaba lista vacía, got %v %v", ps, err)
	}
	p, err := s.Save(ConnectionProfile{Name: "b", Driver: "sqlite"})
	if err != nil || p.ID == "" {
		t.Fatalf("save: %v %q", err, p.ID)
	}
	p.Name = "a"
	s.Save(p)
	s.Save(ConnectionProfile{Name: "z", Driver: "postgres"})
	ps, _ := s.List()
	if len(ps) != 2 || ps[0].Name != "a" {
		t.Fatalf("lista inesperada: %+v", ps)
	}
	if err := s.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(p.ID); err == nil {
		t.Fatal("debería no existir")
	}
}

func TestColorValidation(t *testing.T) {
	s, _ := NewStore(filepath.Join(t.TempDir(), "c.json"))
	for _, ok := range []string{"", "#ff0000", "#A1b2C3"} {
		if _, err := s.Save(ConnectionProfile{Name: "x", Driver: "sqlite", Color: ok}); err != nil {
			t.Errorf("color %q debería ser válido: %v", ok, err)
		}
	}
	for _, bad := range []string{"red", "#fff", "#gggggg", "#ff0000;background:url(x)", "url(x)"} {
		if _, err := s.Save(ConnectionProfile{Name: "x", Driver: "sqlite", Color: bad}); err == nil {
			t.Errorf("color %q debería rechazarse", bad)
		}
	}
	ps, _ := s.List()
	if len(ps) != 3 {
		t.Fatalf("solo deberían guardarse los 3 válidos, hay %d", len(ps))
	}
}
