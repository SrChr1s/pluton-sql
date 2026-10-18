package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func add(t *testing.T, s *Store, profile, sql string) {
	t.Helper()
	if err := s.Add(Entry{Profile: profile, SQL: sql, Database: "db"}); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryAddListAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	s := NewStore(path)
	add(t, s, "p1", "SELECT 1")
	add(t, s, "p1", "SELECT 2")
	add(t, s, "p2", "SELECT 99")

	got, _ := s.List("p1", "", 0)
	if len(got) != 2 || got[0].SQL != "SELECT 2" || got[1].SQL != "SELECT 1" {
		t.Fatalf("debe listar la más reciente primero y solo del perfil: %+v", got)
	}
	// Otra instancia sobre el mismo archivo ve lo mismo (persistencia).
	again, _ := NewStore(path).List("p1", "", 0)
	if len(again) != 2 {
		t.Errorf("no persistió: %+v", again)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("el historial puede contener datos sensibles, permisos = %v (%v)", st.Mode().Perm(), err)
	}
	if f, _ := s.List("p1", "select 1", 0); len(f) != 1 {
		t.Errorf("búsqueda sin distinguir mayúsculas: %+v", f)
	}
	if f, _ := s.List("p1", "", 1); len(f) != 1 || f[0].SQL != "SELECT 2" {
		t.Errorf("límite: %+v", f)
	}
}

func TestHistoryGroupsOnlyConsecutiveDuplicates(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "h.json"))
	add(t, s, "p", "SELECT 1")
	add(t, s, "p", "SELECT 1")
	add(t, s, "p", "SELECT 1")
	if got, _ := s.List("p", "", 0); len(got) != 1 || got[0].Count != 3 {
		t.Fatalf("tres ejecuciones consecutivas idénticas = 1 entrada con Count 3: %+v", got)
	}
	add(t, s, "p", "SELECT 2")
	add(t, s, "p", "SELECT 1") // ya no es consecutiva: entrada nueva
	got, _ := s.List("p", "", 0)
	if len(got) != 3 || got[0].Count != 1 {
		t.Errorf("las no consecutivas no se agrupan: %+v", got)
	}
	// Una ejecución de OTRO perfil entre medias no rompe la agrupación del primero.
	add(t, s, "p", "SELECT 5")
	add(t, s, "q", "SELECT 7")
	add(t, s, "p", "SELECT 5")
	if got, _ := s.List("p", "SELECT 5", 0); len(got) != 1 || got[0].Count != 2 {
		t.Errorf("agrupación por perfil: %+v", got)
	}
}

func TestHistoryIgnoresEmptyAndTrims(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "h.json"))
	add(t, s, "p", "   \n  ")
	s.Add(Entry{SQL: "SELECT 1"}) // sin perfil
	if got, _ := s.List("p", "", 0); len(got) != 0 {
		t.Errorf("SQL vacío: %+v", got)
	}
	add(t, s, "p", "  SELECT 1  \n")
	if got, _ := s.List("p", "", 0); got[0].SQL != "SELECT 1" {
		t.Errorf("debe recortar espacios: %q", got[0].SQL)
	}
}

func TestHistoryCaps(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "h.json"))
	for i := 0; i < MaxPerProfile+25; i++ {
		add(t, s, "grande", fmt.Sprintf("SELECT %d", i))
	}
	add(t, s, "otro", "SELECT 'a'")
	got, _ := s.List("grande", "", 0)
	if len(got) != MaxPerProfile {
		t.Fatalf("debe conservar %d entradas, tiene %d", MaxPerProfile, len(got))
	}
	if got[0].SQL != fmt.Sprintf("SELECT %d", MaxPerProfile+24) || got[len(got)-1].SQL != "SELECT 25" {
		t.Errorf("debe descartar las más antiguas: %q … %q", got[0].SQL, got[len(got)-1].SQL)
	}
	if o, _ := s.List("otro", "", 0); len(o) != 1 {
		t.Error("recortar un perfil no debe tocar a los demás")
	}
}

func TestHistoryTruncatesLongSQLWithoutBreakingUTF8(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "h.json"))
	long := strings.Repeat("ñ", MaxSQLBytes) // 2 bytes por carácter: el corte cae a media letra
	add(t, s, "p", long)
	got, _ := s.List("p", "", 0)
	if len(got[0].SQL) > MaxSQLBytes+40 || !utf8.ValidString(got[0].SQL) || !strings.Contains(got[0].SQL, "truncado") {
		t.Errorf("truncado inválido: %d bytes, utf8=%v", len(got[0].SQL), utf8.ValidString(got[0].SQL))
	}
}

func TestHistoryDeleteAndClear(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "h.json"))
	add(t, s, "p", "SELECT 1")
	add(t, s, "p", "SELECT 2")
	add(t, s, "q", "SELECT 3")
	got, _ := s.List("p", "", 0)
	if err := s.Delete(got[0].ID); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.List("p", "", 0); len(after) != 1 || after[0].SQL != "SELECT 1" {
		t.Errorf("Delete: %+v", after)
	}
	if err := s.Delete("no-existe"); err != nil {
		t.Errorf("borrar algo inexistente no es un error: %v", err)
	}
	s.Clear("p")
	if after, _ := s.List("p", "", 0); len(after) != 0 {
		t.Error("Clear")
	}
	if q, _ := s.List("q", "", 0); len(q) != 1 {
		t.Error("Clear de un perfil no debe borrar otros")
	}
}

func TestHistoryCorruptFileIsReportedNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.json")
	os.WriteFile(path, []byte("{esto no es json"), 0o600)
	s := NewStore(path)
	if err := s.Add(Entry{Profile: "p", SQL: "SELECT 1"}); err == nil {
		t.Fatal("un historial corrupto debe dar error")
	}
	if data, _ := os.ReadFile(path); string(data) != "{esto no es json" {
		t.Error("no debe sobrescribir un archivo que no pudo leer (se perdería el historial)")
	}
}

func TestSavedQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	s := NewSavedStore(path)
	a, err := s.Save(Saved{Name: "Clientes activos", SQL: "SELECT * FROM c", Driver: "postgres"})
	if err != nil || a.ID == "" {
		t.Fatalf("Save: %v %+v", err, a)
	}
	s.Save(Saved{Name: "abajo", SQL: "SELECT 2"})
	list, _ := s.List()
	if len(list) != 2 || list[0].Name != "abajo" {
		t.Errorf("orden alfabético sin distinguir mayúsculas: %+v", list)
	}
	if _, err := s.Save(Saved{Name: "CLIENTES ACTIVOS", SQL: "x"}); err == nil {
		t.Error("el nombre no debe repetirse (ignorando mayúsculas)")
	}
	// Actualizar la misma consulta con su nombre no es un duplicado.
	a.SQL = "SELECT * FROM clientes"
	if b, err := s.Save(a); err != nil || b.ID != a.ID {
		t.Errorf("actualizar: %v", err)
	}
	if _, err := s.Save(Saved{Name: " ", SQL: "x"}); err == nil {
		t.Error("nombre vacío")
	}
	if _, err := s.Save(Saved{Name: "n", SQL: "  "}); err == nil {
		t.Error("SQL vacío")
	}
	if again, _ := NewSavedStore(path).List(); len(again) != 2 || again[1].SQL != "SELECT * FROM clientes" {
		t.Errorf("persistencia: %+v", again)
	}
	s.Delete(a.ID)
	if left, _ := s.List(); len(left) != 1 {
		t.Errorf("Delete: %+v", left)
	}
}
