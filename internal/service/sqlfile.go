package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxSQLFileBytes evita cargar en el editor un archivo enorme por error (p. ej. un volcado de varios GB).
const MaxSQLFileBytes = 10 << 20

// SQLFile es el contenido de un archivo .sql abierto.
type SQLFile struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"` // siempre con \n; CRLF indica que el original usaba \r\n
	CRLF    bool   `json:"crlf"`
}

// ReadSQLFile lee un archivo de texto UTF-8 (con o sin BOM) normalizando el fin de línea a \n.
func ReadSQLFile(path string) (*SQLFile, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%s es una carpeta", path)
	}
	if st.Size() > MaxSQLFileBytes {
		return nil, fmt.Errorf("el archivo pesa %d MB; el límite para abrirlo en el editor es %d MB", st.Size()>>20, MaxSQLFileBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = []byte(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		return nil, errors.New("el archivo no está en UTF-8: abrirlo corrompería los caracteres especiales")
	}
	text := string(data)
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	return &SQLFile{Path: path, Name: filepath.Base(path), Content: text, CRLF: crlf}, nil
}

// WriteSQLFile guarda content de forma atómica (archivo temporal + rename), conservando los permisos del
// original y siguiendo enlaces simbólicos. Si crlf, el fin de línea vuelve a ser \r\n.
func WriteSQLFile(path, content string, crlf bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("falta la ruta del archivo")
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if crlf {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		if st.IsDir() {
			return fmt.Errorf("%s es una carpeta", path)
		}
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pluton-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op tras el rename
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
