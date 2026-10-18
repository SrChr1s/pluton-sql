// Command tlsoverlay genera un overlay de `go build` que desactiva la
// mitigación BEAST (registro de 1 byte) de crypto/tls en TLS 1.0 + CBC.
//
// SQL Server de la era 2005/2008 corta la conexión al recibir ese primer
// registro de 1 byte (Java necesita -Djsse.enableCBCProtection=false por lo
// mismo). Go no ofrece ningún ajuste para desactivarlo.
//
// Uso: go run ./tools/tlsoverlay   →   build/_overlay/overlay.json
// Luego: GOFLAGS=-overlay=$PWD/build/_overlay/overlay.json wails build ...
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	needle = "if len(b) > 1 && c.vers == VersionTLS10 {"
	patch  = "if false && len(b) > 1 && c.vers == VersionTLS10 { // parcheado por pluton-sql: sin registro de 1 byte"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tlsoverlay:", err)
		os.Exit(1)
	}
}

func run() error {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	src := filepath.Join(strings.TrimSpace(string(out)), "src", "crypto", "tls", "conn.go")
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if strings.Count(string(data), needle) != 1 {
		return fmt.Errorf("no encuentro el bloque BEAST en %s (¿cambió tu versión de Go?); revisa tools/tlsoverlay", src)
	}
	dir, _ := filepath.Abs(filepath.Join("build", "_overlay"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	patched := filepath.Join(dir, "conn.go")
	if err := os.WriteFile(patched, []byte(strings.Replace(string(data), needle, patch, 1)), 0o644); err != nil {
		return err
	}
	js, _ := json.MarshalIndent(map[string]any{"Replace": map[string]string{src: patched}}, "", "  ")
	return os.WriteFile(filepath.Join(dir, "overlay.json"), js, 0o644)
}
