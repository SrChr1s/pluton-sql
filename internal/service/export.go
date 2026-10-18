package service

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/transfer"
)

// ExportRequest describe qué exportar y cómo. Kind "query" exporta el resultado de SQL; "table" exporta
// una tabla (con filtro y orden opcionales). Siempre se reejecuta en el servidor: no hay límite de filas.
type ExportRequest struct {
	Profile  string `json:"profile"`
	Database string `json:"database"`
	Kind     string `json:"kind"`
	SQL      string `json:"sql"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Filter   string `json:"filter"`
	OrderBy  string `json:"orderBy"`
	Desc     bool   `json:"desc"`

	Format    string `json:"format"` // csv | json | sql
	Path      string `json:"path"`
	Delimiter string `json:"delimiter"` // CSV; "\t" = tabulador
	Header    bool   `json:"header"`    // CSV
	BOM       bool   `json:"bom"`       // CSV: UTF-8 con BOM (Excel)
	SQLTable  string `json:"sqlTable"`  // INSERT: nombre de la tabla destino cuando Kind = query
}

type ExportResult struct {
	Path       string `json:"path"`
	Rows       int64  `json:"rows"`
	Bytes      int64  `json:"bytes"`
	DurationMs int64  `json:"durationMs"`
	Cancelled  bool   `json:"cancelled"`
}

type ExportService struct {
	pools  Pools
	events *Events
	jobs   jobs
}

func NewExportService(pools Pools, events *Events) *ExportService {
	return &ExportService{pools: pools, events: events}
}

// Cancel interrumpe una exportación en curso.
func (s *ExportService) Cancel(id string) { s.jobs.cancel(id) }

func parseDelimiter(d string) rune {
	switch d {
	case "", ",":
		return ','
	case "\\t", "\t", "tab":
		return '\t'
	}
	return []rune(d)[0]
}

// Run exporta y devuelve un resumen. El destino solo se reemplaza si la exportación termina entera.
func (s *ExportService) Run(id string, req ExportRequest) (*ExportResult, error) {
	if req.Path == "" {
		return nil, errors.New("falta el archivo de destino")
	}
	if req.Format != "csv" && req.Format != "json" && req.Format != "sql" {
		return nil, fmt.Errorf("formato no soportado: %q", req.Format)
	}
	if st, err := os.Stat(filepath.Dir(req.Path)); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("la carpeta de destino no existe: %s", filepath.Dir(req.Path))
	}
	ctx, done := s.jobs.start(id)
	defer done()

	db, d, err := s.pools.Get(ctx, req.Profile, req.Database)
	if err != nil {
		return nil, err
	}

	var query, target string
	switch req.Kind {
	case "table":
		_, _, _, meta, err := openTable(ctx, s.pools, TableRef{Profile: req.Profile, Database: req.Database, Schema: req.Schema, Table: req.Table})
		if err != nil {
			return nil, err
		}
		keys, hasPK := keyColumns(meta, d.Name())
		order, err := orderClause(d, meta, req.OrderBy, req.Desc, keys, hasPK)
		if err != nil {
			return nil, err
		}
		where, err := whereClause(req.Filter)
		if err != nil {
			return nil, err
		}
		target = qualified(d, TableRef{Schema: req.Schema, Table: req.Table})
		query = "SELECT * FROM " + target + where
		if order != "" {
			query += " " + order
		}
	case "query":
		query = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(req.SQL), ";"))
		if !returnsRows(query) {
			return nil, errors.New("solo se pueden exportar consultas que devuelven filas (SELECT, WITH…)")
		}
		name := strings.TrimSpace(req.SQLTable)
		if name == "" {
			name = "tabla"
		}
		parts := strings.Split(name, ".")
		for i, p := range parts {
			parts[i] = d.Quote(p)
		}
		target = strings.Join(parts, ".")
	default:
		return nil, fmt.Errorf("tipo de exportación no soportado: %q", req.Kind)
	}

	start := time.Now()
	res := &ExportResult{Path: req.Path}
	// Un error que llega con el contexto ya cancelado es consecuencia de la cancelación, no un fallo.
	fail := func(err error) (*ExportResult, error) {
		if ctx.Err() != nil {
			res.Cancelled = true
			return res, nil
		}
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fail(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return fail(err)
	}

	part := req.Path + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(part) // ni cancelar ni fallar deja un archivo a medias
		}
	}()
	bw := bufio.NewWriterSize(f, 256<<10)

	var w transfer.Writer
	switch req.Format {
	case "csv":
		w = transfer.NewCSV(bw, transfer.CSVOptions{Delimiter: parseDelimiter(req.Delimiter), Header: req.Header, BOM: req.BOM})
	case "json":
		w = transfer.NewJSON(bw)
	case "sql":
		w = transfer.NewSQL(bw, d, target, 100)
	}
	if err := w.Begin(cols); err != nil {
		return fail(err)
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	lastEmit := time.Now()
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return fail(err)
		}
		if err := w.Row(vals); err != nil {
			return fail(err)
		}
		res.Rows++
		if res.Rows%1000 == 0 && time.Since(lastEmit) > 250*time.Millisecond {
			s.events.Emit("export:progress", map[string]any{"id": id, "rows": res.Rows})
			lastEmit = time.Now()
		}
	}
	if err := rows.Err(); err != nil {
		return fail(err)
	}
	if ctx.Err() != nil {
		res.Cancelled = true
		return res, nil
	}
	if err := w.End(); err != nil {
		return nil, err
	}
	if err := bw.Flush(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(part, req.Path); err != nil {
		return nil, err
	}
	ok = true
	if st, err := os.Stat(req.Path); err == nil {
		res.Bytes = st.Size()
	}
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}
