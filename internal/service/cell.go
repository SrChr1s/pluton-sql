package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	maxImagePreview = 12 << 20 // más que esto no se manda al webview como data URL
	hexDumpBytes    = 512
	textPreviewMax  = 4096
	maxSniffOffset  = 128 // cabeceras (p. ej. OLE de Access/Northwind) antepuestas a la imagen
)

// CellRequest identifica una celda: la columna de la fila con esa clave.
type CellRequest struct {
	Table  TableRef       `json:"table"`
	Column string         `json:"column"`
	Key    map[string]any `json:"key"`
}

// CellPreview es lo necesario para mostrar el contenido de una celda binaria.
type CellPreview struct {
	Size int    `json:"size"`
	Mime string `json:"mime"`
	Kind string `json:"kind"` // image | text | binary
	// Offset son los bytes de cabecera que preceden a la imagen (se omiten al guardar).
	Offset int `json:"offset"`
	// Image es una data URL lista para <img src>; vacía si no es imagen o es demasiado grande.
	Image string `json:"image"`
	// Text es el comienzo del contenido si es texto UTF-8; Hex, un volcado de los primeros bytes.
	Text string `json:"text"`
	Hex  string `json:"hex"`
}

// cellBytes lee el valor completo de una celda por la clave de su fila.
func (s *DataService) cellBytes(ctx context.Context, req CellRequest) ([]byte, error) {
	_, db, d, meta, err := s.open(ctx, req.Table)
	if err != nil {
		return nil, err
	}
	found := false
	for _, c := range meta {
		found = found || c.Name == req.Column
	}
	if !found {
		return nil, fmt.Errorf("la columna %q no existe", req.Column)
	}
	keys, _ := keyColumns(meta, d.Name())
	var args []any
	where, err := keyWhere(d, keys, req.Key, func(v any) string {
		args = append(args, coerce(v))
		return d.Param(len(args))
	})
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SELECT "+d.Quote(req.Column)+" FROM "+qualified(d, req.Table)+" WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("la fila ya no existe (¿cambió desde que se cargó?)")
	}
	var v any
	if err := rows.Scan(&v); err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, errors.New("la clave no identifica una sola fila")
	}
	switch x := v.(type) {
	case nil:
		return nil, errors.New("la celda es NULL")
	case []byte:
		return x, nil
	case string:
		return []byte(x), nil
	default:
		return []byte(fmt.Sprint(x)), nil
	}
}

// sniffImage busca una imagen al comienzo del valor o tras una cabecera corta.
func sniffImage(b []byte) (mime string, off int) {
	for off = 0; off <= maxSniffOffset && off < len(b); off++ {
		end := min(off+hexDumpBytes, len(b))
		if m := http.DetectContentType(b[off:end]); strings.HasPrefix(m, "image/") {
			return m, off
		}
	}
	return "", 0
}

var imageExt = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp",
	"image/bmp": ".bmp", "image/x-icon": ".ico",
}

func hexDump(b []byte) string {
	b = b[:min(len(b), hexDumpBytes)]
	var sb strings.Builder
	for i := 0; i < len(b); i += 16 {
		row := b[i:min(i+16, len(b))]
		fmt.Fprintf(&sb, "%06x  %-47s  ", i, strings.TrimSpace(spaced(hex.EncodeToString(row))))
		for _, c := range row {
			if c >= 0x20 && c < 0x7f {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func spaced(h string) string {
	var sb strings.Builder
	for i := 0; i+1 < len(h); i += 2 {
		sb.WriteString(h[i:i+2] + " ")
	}
	return sb.String()
}

// CellPreview lee una celda binaria y devuelve lo necesario para mostrarla.
func (s *DataService) CellPreview(req CellRequest) (*CellPreview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	b, err := s.cellBytes(ctx, req)
	if err != nil {
		return nil, err
	}
	p := &CellPreview{Size: len(b), Kind: "binary", Mime: "application/octet-stream", Hex: hexDump(b)}
	if m, off := sniffImage(b); m != "" {
		p.Kind, p.Mime, p.Offset = "image", m, off
		if len(b)-off <= maxImagePreview {
			p.Image = "data:" + m + ";base64," + base64.StdEncoding.EncodeToString(b[off:])
		}
	} else if utf8.Valid(b) {
		p.Kind, p.Mime = "text", "text/plain"
		t := b
		if len(t) > textPreviewMax {
			t = t[:textPreviewMax]
			for len(t) > 0 && !utf8.Valid(t) { // no cortar un carácter por la mitad
				t = t[:len(t)-1]
			}
		}
		p.Text = string(t)
	}
	return p, nil
}

// CellSave pregunta dónde guardar el contenido de una celda y lo escribe. Devuelve la ruta,
// o "" si se cancela. Una imagen con cabecera antepuesta se guarda sin ella para que se pueda abrir.
func (s *DataService) CellSave(req CellRequest, defaultName string) (string, error) {
	if s.ctx == nil {
		return "", errors.New("diálogos no disponibles")
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	b, err := s.cellBytes(ctx, req)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(defaultName)
	if name == "" {
		name = req.Column
	}
	if m, off := sniffImage(b); m != "" {
		b = b[off:]
		if !strings.Contains(name, ".") {
			name += imageExt[m]
		}
	} else if !strings.Contains(name, ".") {
		name += ".bin"
	}
	path, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{Title: "Guardar contenido de la celda", DefaultFilename: name})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
}
