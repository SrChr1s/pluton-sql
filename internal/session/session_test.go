package session

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/SrChr1s/pluton-sql/internal/config"
	_ "modernc.org/sqlite"
)

func memDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCloseDatabase(t *testing.T) {
	store, _ := config.NewStore(filepath.Join(t.TempDir(), "c.json"))
	p, _ := store.Save(config.ConnectionProfile{Name: "x", Driver: "postgres", Database: "app"})
	m := NewManager(store)
	app, other, def := memDB(t), memDB(t), memDB(t)
	m.pools[key(p.ID, "app")] = entry{db: app}
	m.pools[key(p.ID, "other")] = entry{db: other}
	m.pools[key(p.ID, "")] = entry{db: def}

	m.CloseDatabase(p.ID, "other")
	if _, ok := m.pools[key(p.ID, "other")]; ok || other.Ping() == nil {
		t.Fatal("el pool de 'other' debería estar cerrado")
	}
	if _, ok := m.pools[key(p.ID, "app")]; !ok || app.Ping() != nil {
		t.Fatal("el pool de 'app' no debería tocarse")
	}
	if _, ok := m.pools[key(p.ID, "")]; !ok {
		t.Fatal("cerrar otra base no debe cerrar el pool por defecto")
	}

	// La base por defecto del perfil también cierra el pool sin nombre.
	m.CloseDatabase(p.ID, "app")
	if len(m.pools) != 0 {
		t.Fatalf("deberían estar todos cerrados, quedan %d", len(m.pools))
	}
	if app.Ping() == nil || def.Ping() == nil {
		t.Fatal("los pools deberían estar cerrados")
	}
}
