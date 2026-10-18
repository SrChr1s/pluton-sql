package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSQLFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	f, err := ReadSQLFile(write("a.sql", []byte("SELECT 1;\nSELECT 'ñandú';\n")))
	if err != nil || f.Name != "a.sql" || f.CRLF || f.Content != "SELECT 1;\nSELECT 'ñandú';\n" {
		t.Fatalf("lectura básica: %v %+v", err, f)
	}
	f, _ = ReadSQLFile(write("crlf.sql", []byte("SELECT 1;\r\nSELECT 2;\r\n")))
	if !f.CRLF || strings.Contains(f.Content, "\r") || f.Content != "SELECT 1;\nSELECT 2;\n" {
		t.Errorf("CRLF debe detectarse y normalizarse: %+v", f)
	}
	f, _ = ReadSQLFile(write("bom.sql", append([]byte{0xEF, 0xBB, 0xBF}, "SELECT 1;"...)))
	if f.Content != "SELECT 1;" {
		t.Errorf("el BOM debe quitarse: %q", f.Content)
	}
	if _, err := ReadSQLFile(write("latin1.sql", []byte("SELECT 'a\xf1o';"))); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("un archivo que no es UTF-8 debe rechazarse con un mensaje claro: %v", err)
	}
	if _, err := ReadSQLFile(dir); err == nil {
		t.Error("una carpeta no es un script")
	}
	if _, err := ReadSQLFile(filepath.Join(dir, "no-existe.sql")); err == nil {
		t.Error("un archivo inexistente debe dar error")
	}
	big := write("big.sql", make([]byte, MaxSQLFileBytes+1))
	if _, err := ReadSQLFile(big); err == nil || !strings.Contains(err.Error(), "límite") {
		t.Errorf("el límite de tamaño: %v", err)
	}
}

func TestWriteSQLFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.sql")

	if err := WriteSQLFile(p, "SELECT 1;\nSELECT 2;\n", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "SELECT 1;\nSELECT 2;\n" {
		t.Errorf("contenido: %q", got)
	}
	// Ida y vuelta de un archivo CRLF: se vuelve a guardar con \r\n y no se "normaliza" a escondidas.
	if err := WriteSQLFile(p, "a\nb\n", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "a\r\nb\r\n" {
		t.Errorf("CRLF: %q", got)
	}
	rt, _ := ReadSQLFile(p)
	if !rt.CRLF || rt.Content != "a\nb\n" {
		t.Errorf("ida y vuelta: %+v", rt)
	}
	// Conserva los permisos del original.
	os.Chmod(p, 0o600)
	WriteSQLFile(p, "x", false)
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("permisos = %v, debía conservar 0600", st.Mode().Perm())
	}
	// No deja archivos temporales.
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("quedaron archivos temporales: %v", ents)
	}
	// Escribe sobre el destino de un enlace simbólico en vez de reemplazar el enlace.
	real := filepath.Join(dir, "real.sql")
	os.WriteFile(real, []byte("viejo"), 0o644)
	link := filepath.Join(dir, "enlace.sql")
	if err := os.Symlink(real, link); err == nil {
		WriteSQLFile(link, "nuevo", false)
		if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
			t.Error("el enlace simbólico fue reemplazado por un archivo")
		}
		if got, _ := os.ReadFile(real); string(got) != "nuevo" {
			t.Errorf("el destino real no se actualizó: %q", got)
		}
	}
	if err := WriteSQLFile("", "x", false); err == nil {
		t.Error("ruta vacía")
	}
	if err := WriteSQLFile(dir, "x", false); err == nil {
		t.Error("no se puede escribir sobre una carpeta")
	}
	if err := WriteSQLFile(filepath.Join(dir, "no", "existe", "s.sql"), "x", false); err == nil {
		t.Error("un directorio inexistente debe dar error")
	}
}
