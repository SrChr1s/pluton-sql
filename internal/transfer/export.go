// Package transfer contiene los escritores de exportación (CSV, JSON, INSERT) y el lector de CSV para
// importar. No depende de la base de datos: recibe valores tal como los entrega database/sql.
package transfer

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Writer recibe un resultado fila a fila.
type Writer interface {
	Begin(cols []string) error
	Row(vals []any) error
	End() error
}

// text devuelve el texto de un valor que database/sql entregó como []byte o string; ok=false si son
// bytes que no son UTF-8 válido (binario real).
func text(v any) (s string, ok bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		if utf8.Valid(x) {
			return string(x), true
		}
	}
	return "", false
}

func hexOf(b []byte) string { return hex.EncodeToString(b) }

// dedupe hace únicos los nombres de columna (SELECT a.id, b.id): id, id_2, id_3…
func dedupe(cols []string) []string {
	seen := map[string]int{}
	out := make([]string, len(cols))
	for i, c := range cols {
		seen[c]++
		if seen[c] > 1 {
			c = fmt.Sprintf("%s_%d", c, seen[c])
		}
		out[i] = c
	}
	return out
}

// ---------------- CSV ----------------

type CSVOptions struct {
	Delimiter rune // coma si es 0
	Header    bool
	BOM       bool // UTF-8 con BOM: Excel lo necesita para leer bien los acentos
}

type csvWriter struct {
	w   io.Writer
	c   *csv.Writer
	o   CSVOptions
	rec []string
}

func NewCSV(w io.Writer, o CSVOptions) Writer {
	if o.Delimiter == 0 {
		o.Delimiter = ','
	}
	c := csv.NewWriter(w)
	c.Comma = o.Delimiter
	return &csvWriter{w: w, c: c, o: o}
}

func csvCell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		if s, ok := text(x); ok {
			return s
		}
		return "0x" + hexOf(x)
	case time.Time:
		// Con espacio y sin "T"/"Z": es lo que aceptan al reimportar PostgreSQL, MySQL y SQL Server (MySQL
		// rechaza 2026-01-02T03:04:05Z) y lo que Excel reconoce como fecha. El desfase solo se escribe si existe.
		if _, off := x.Zone(); off == 0 {
			return x.Format("2006-01-02 15:04:05.999999")
		}
		return x.Format("2006-01-02 15:04:05.999999-07:00")
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return ""
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return fmt.Sprint(v)
}

func (c *csvWriter) Begin(cols []string) error {
	if c.o.BOM {
		if _, err := io.WriteString(c.w, "\xef\xbb\xbf"); err != nil {
			return err
		}
	}
	c.rec = make([]string, len(cols))
	if c.o.Header {
		return c.c.Write(dedupe(cols))
	}
	return nil
}

func (c *csvWriter) Row(vals []any) error {
	for i, v := range vals {
		c.rec[i] = csvCell(v)
	}
	return c.c.Write(c.rec)
}

func (c *csvWriter) End() error {
	c.c.Flush()
	return c.c.Error()
}

// ---------------- JSON ----------------

type jsonWriter struct {
	w     io.Writer
	keys  [][]byte // nombres ya serializados
	count int
}

func NewJSON(w io.Writer) Writer { return &jsonWriter{w: w} }

func jsonValue(v any) any {
	switch x := v.(type) {
	case []byte:
		if s, ok := text(x); ok {
			return s
		}
		return "0x" + hexOf(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil // JSON no tiene NaN ni Infinity
		}
	}
	return v
}

func (j *jsonWriter) Begin(cols []string) error {
	for _, c := range dedupe(cols) {
		k, err := json.Marshal(c)
		if err != nil {
			return err
		}
		j.keys = append(j.keys, k)
	}
	_, err := io.WriteString(j.w, "[")
	return err
}

func (j *jsonWriter) Row(vals []any) error {
	var b bytes.Buffer
	if j.count > 0 {
		b.WriteByte(',')
	}
	b.WriteString("\n  {")
	for i, v := range vals {
		if i > 0 {
			b.WriteString(", ")
		}
		enc, err := json.Marshal(jsonValue(v)) // los enteros salen exactos, sin pasar por float64
		if err != nil {
			return err
		}
		b.Write(j.keys[i])
		b.WriteString(": ")
		b.Write(enc)
	}
	b.WriteByte('}')
	j.count++
	_, err := j.w.Write(b.Bytes())
	return err
}

func (j *jsonWriter) End() error {
	end := "\n]\n"
	if j.count == 0 {
		end = "]\n"
	}
	_, err := io.WriteString(j.w, end)
	return err
}

// ---------------- INSERT ----------------

// SQLQuoter cita identificadores (lo implementa driver.Dialect).
type SQLQuoter interface {
	Name() string
	Quote(ident string) string
}

type sqlWriter struct {
	w      io.Writer
	q      SQLQuoter
	engine string
	table  string // ya citado y cualificado
	cols   string
	batch  int
	rows   []string
}

// NewSQL escribe INSERT por lotes. table debe venir ya citado; batch es el nº de filas por sentencia.
func NewSQL(w io.Writer, q SQLQuoter, table string, batch int) Writer {
	if batch <= 0 {
		batch = 100
	}
	return &sqlWriter{w: w, q: q, engine: q.Name(), table: table, batch: batch}
}

func (s *sqlWriter) Begin(cols []string) error {
	quoted := make([]string, len(cols))
	for i, c := range dedupe(cols) {
		quoted[i] = s.q.Quote(c)
	}
	s.cols = strings.Join(quoted, ", ")
	return nil
}

// Literal devuelve el valor como literal SQL para el motor dado.
func Literal(engine string, v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "NULL"
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if engine == "postgres" || engine == "mysql" {
			return strings.ToUpper(strconv.FormatBool(x))
		}
		if x {
			return "1"
		}
		return "0"
	case time.Time:
		layout := "2006-01-02 15:04:05.999999"
		if engine == "postgres" {
			layout = "2006-01-02 15:04:05.999999Z07:00"
		}
		return "'" + x.Format(layout) + "'"
	case []byte:
		if s, ok := text(x); ok {
			return stringLiteral(engine, s)
		}
		switch engine {
		case "postgres":
			return `'\x` + hexOf(x) + `'`
		case "mssql":
			return "0x" + hexOf(x)
		}
		return "X'" + hexOf(x) + "'"
	case string:
		return stringLiteral(engine, x)
	}
	return stringLiteral(engine, fmt.Sprint(v))
}

func stringLiteral(engine, s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	switch engine {
	case "mysql":
		s = strings.ReplaceAll(s, `\`, `\\`) // MySQL trata la barra invertida como escape
	case "mssql":
		return "N'" + s + "'" // conserva los caracteres Unicode en columnas nvarchar
	}
	return "'" + s + "'"
}

func (s *sqlWriter) Row(vals []any) error {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = Literal(s.engine, v)
	}
	s.rows = append(s.rows, "  ("+strings.Join(parts, ", ")+")")
	if len(s.rows) >= s.batch {
		return s.flush()
	}
	return nil
}

func (s *sqlWriter) flush() error {
	if len(s.rows) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(s.w, "INSERT INTO %s (%s) VALUES\n%s;\n", s.table, s.cols, strings.Join(s.rows, ",\n"))
	s.rows = s.rows[:0]
	return err
}

func (s *sqlWriter) End() error { return s.flush() }
