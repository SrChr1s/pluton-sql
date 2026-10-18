package transfer

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Candidatos a separador, por orden de preferencia.
var delimiters = []rune{',', ';', '\t', '|'}

// DetectDelimiter elige el separador que aparece el mismo número (>0) de veces en las primeras líneas
// y, si hay varios, el que más aparece. Ante la duda devuelve la coma.
func DetectDelimiter(sample string) rune {
	lines := []string{}
	for _, l := range strings.Split(sample, "\n") {
		if l = strings.TrimRight(l, "\r"); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
		if len(lines) == 5 {
			break
		}
	}
	best, bestN := ',', 0
	for _, d := range delimiters {
		n := 0
		if len(lines) > 0 {
			n = countOutsideQuotes(lines[0], d)
		}
		consistent := n > 0
		for _, l := range lines {
			if countOutsideQuotes(l, d) != n {
				consistent = false
				break
			}
		}
		if consistent && n > bestN {
			best, bestN = d, n
		}
	}
	return best
}

// countOutsideQuotes cuenta d en la línea ignorando lo que va entre comillas: un separador dentro de un
// campo entrecomillado ("Luis; el grande") no separa columnas. Las comillas dobles ("") alternan dos veces.
func countOutsideQuotes(line string, d rune) int {
	n, in := 0, false
	for _, r := range line {
		switch {
		case r == '"':
			in = !in
		case r == d && !in:
			n++
		}
	}
	return n
}

// Reader lee un CSV como flujo de registros.
type Reader struct {
	f      *os.File
	r      *csv.Reader
	Header []string // vacío si el archivo no tiene cabecera
	line   int
	hasHdr bool
}

// OpenCSV abre path. delim 0 = detectarlo. Si hasHeader, la primera fila se lee como cabecera.
func OpenCSV(path string, delim rune, hasHeader bool) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(f)
	if b, _ := br.Peek(3); len(b) == 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		br.Discard(3) // BOM UTF-8
	}
	if delim == 0 {
		sample, _ := br.Peek(8192)
		if i := strings.LastIndexByte(string(sample), '\n'); i > 0 && len(sample) == 8192 {
			sample = sample[:i] // no usar una última línea cortada
		}
		delim = DetectDelimiter(string(sample))
	}
	c := csv.NewReader(br)
	c.Comma = delim
	c.FieldsPerRecord = -1 // la longitud de cada fila la comprueba quien importa, con su número de fila
	c.ReuseRecord = false
	r := &Reader{f: f, r: c, hasHdr: hasHeader}
	if hasHeader {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			f.Close()
			return nil, errors.New("el archivo está vacío")
		}
		if err != nil {
			f.Close()
			return nil, err
		}
		r.Header = rec
	}
	return r, nil
}

func (r *Reader) delimiterRune() rune { return r.r.Comma }

// Next devuelve el siguiente registro. Línea es el nº de registro del archivo (la cabecera cuenta como 1).
func (r *Reader) Next() ([]string, error) {
	rec, err := r.r.Read()
	if err != nil {
		return nil, err
	}
	r.line++
	for _, f := range rec {
		if !utf8.ValidString(f) {
			return nil, fmt.Errorf("fila %d: el archivo no está en UTF-8 (conviértelo antes de importar)", r.line)
		}
	}
	return rec, nil
}

// Line es el número del último registro leído (la cabecera cuenta como el 1).
func (r *Reader) Line() int { return r.line }

func (r *Reader) Close() error { return r.f.Close() }

// Preview es lo que muestra el asistente antes de importar.
type Preview struct {
	Delimiter string     `json:"delimiter"`
	Header    []string   `json:"header"` // nombres de columna (generados si el archivo no tiene cabecera)
	Rows      [][]string `json:"rows"`
	Columns   int        `json:"columns"`
	Warnings  []string   `json:"warnings"`
}

// ReadPreview lee las primeras maxRows filas de datos.
func ReadPreview(path string, delim rune, hasHeader bool, maxRows int) (*Preview, error) {
	r, err := OpenCSV(path, delim, hasHeader)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	p := &Preview{Delimiter: string(r.delimiterRune()), Rows: [][]string{}, Warnings: []string{}}
	cols := len(r.Header)
	ragged := false
	for len(p.Rows) < maxRows {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		p.Rows = append(p.Rows, rec)
		if cols == 0 {
			cols = len(rec)
		} else if len(rec) != cols {
			ragged = true
		}
		cols = max(cols, len(rec))
	}
	if hasHeader && len(p.Rows) == 0 {
		p.Warnings = append(p.Warnings, "el archivo solo tiene la cabecera, sin filas de datos")
	}
	if ragged {
		p.Warnings = append(p.Warnings, "algunas filas tienen un número de columnas distinto al de la cabecera: revisa el separador")
	}
	p.Columns = cols
	p.Header = r.Header
	for len(p.Header) < cols {
		p.Header = append(p.Header, fmt.Sprintf("columna%d", len(p.Header)+1))
	}
	if p.Header == nil {
		p.Header = []string{}
	}
	return p, nil
}
