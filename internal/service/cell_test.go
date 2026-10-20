package service

import (
	"strings"
	"testing"
)

func TestCellPreview(t *testing.T) {
	s, db := newData(t)
	if _, err := db.Exec(`CREATE TABLE fotos (id INTEGER PRIMARY KEY, img BLOB)`); err != nil {
		t.Fatal(err)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 300)...)
	ole := append(make([]byte, 78), []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")...) // JPEG tras una cabecera
	for i, v := range [][]byte{png, ole, {0xde, 0xad, 0x00, 0xbe}, []byte("hola"), nil} {
		if _, err := db.Exec(`INSERT INTO fotos VALUES (?, ?)`, i+1, v); err != nil {
			t.Fatal(err)
		}
	}
	get := func(id int) (*CellPreview, error) {
		return s.CellPreview(CellRequest{Table: ref("fotos"), Column: "img", Key: map[string]any{"id": float64(id)}})
	}

	p, err := get(1)
	if err != nil || p.Kind != "image" || p.Mime != "image/png" || p.Size != len(png) || !strings.HasPrefix(p.Image, "data:image/png;base64,") {
		t.Errorf("png: %+v %v", p, err)
	}
	if p, err = get(2); err != nil || p.Kind != "image" || p.Mime != "image/jpeg" || p.Offset != 78 {
		t.Errorf("jpeg con cabecera: %+v %v", p, err)
	}
	if p, err = get(3); err != nil || p.Kind != "binary" || p.Image != "" || !strings.Contains(p.Hex, "de ad 00 be") {
		t.Errorf("binario: %+v %v", p, err)
	}
	if p, err = get(4); err != nil || p.Kind != "text" || p.Text != "hola" {
		t.Errorf("texto: %+v %v", p, err)
	}
	if _, err = get(5); err == nil {
		t.Error("NULL debería dar error")
	}
	if _, err = get(99); err == nil {
		t.Error("fila inexistente debería dar error")
	}
	if _, err = s.CellPreview(CellRequest{Table: ref("fotos"), Column: "nada", Key: map[string]any{"id": 1.0}}); err == nil {
		t.Error("columna inexistente debería dar error")
	}
	if _, err = s.CellPreview(CellRequest{Table: ref("fotos"), Column: "img", Key: map[string]any{}}); err == nil {
		t.Error("sin clave debería dar error")
	}
}
