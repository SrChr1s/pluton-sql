package service

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/transfer"
)

// ImportMap asocia una columna de la tabla con una columna del CSV. Source < 0 = no se importa
// (la tabla usa su valor por defecto).
type ImportMap struct {
	Column string `json:"column"`
	Source int    `json:"source"`
}

type ImportRequest struct {
	Table       TableRef    `json:"table"`
	Path        string      `json:"path"`
	Delimiter   string      `json:"delimiter"` // vacío = detectarlo
	HasHeader   bool        `json:"hasHeader"`
	Mapping     []ImportMap `json:"mapping"`
	EmptyAsNull bool        `json:"emptyAsNull"` // cadena vacía = NULL también en columnas de texto
}

type ImportResult struct {
	Inserted   int64 `json:"inserted"`
	DurationMs int64 `json:"durationMs"`
	Cancelled  bool  `json:"cancelled"`
}

type ImportService struct {
	pools  Pools
	events *Events
	jobs   jobs
}

func NewImportService(pools Pools, events *Events) *ImportService {
	return &ImportService{pools: pools, events: events}
}

// Cancel interrumpe una importación en curso (se revierte entera).
func (s *ImportService) Cancel(id string) { s.jobs.cancel(id) }

// Preview lee el comienzo del CSV: separador detectado, cabecera y primeras filas.
func (s *ImportService) Preview(path, delimiter string, hasHeader bool) (*transfer.Preview, error) {
	var d rune
	if delimiter != "" {
		d = parseDelimiter(delimiter)
	}
	return transfer.ReadPreview(path, d, hasHeader, 20)
}

var textualType = regexp.MustCompile(`(?i)char|text|string|clob|enum|set`)

// Run importa el CSV en una transacción: si cualquier fila falla no se inserta ninguna.
func (s *ImportService) Run(id string, req ImportRequest) (*ImportResult, error) {
	ctx, done := s.jobs.start(id)
	defer done()

	t, db, d, meta, err := openTable(ctx, s.pools, req.Table)
	if err != nil {
		return nil, err
	}
	if t.Type == "view" {
		return nil, errors.New("no se puede importar en una vista")
	}
	types := map[string]string{}
	for _, c := range meta {
		types[c.Name] = c.Type
	}

	var cols []string
	var srcs []int
	seen := map[string]bool{}
	maxSrc := -1
	for _, m := range req.Mapping {
		if m.Source < 0 {
			continue
		}
		if _, ok := types[m.Column]; !ok {
			return nil, fmt.Errorf("la columna %q no existe en la tabla", m.Column)
		}
		if seen[m.Column] {
			return nil, fmt.Errorf("la columna %q está mapeada dos veces", m.Column)
		}
		seen[m.Column] = true
		cols = append(cols, m.Column)
		srcs = append(srcs, m.Source)
		maxSrc = max(maxSrc, m.Source)
	}
	if len(cols) == 0 {
		return nil, errors.New("no hay ninguna columna mapeada: elige al menos una")
	}

	var delim rune
	if req.Delimiter != "" {
		delim = parseDelimiter(req.Delimiter)
	}
	r, err := transfer.OpenCSV(req.Path, delim, req.HasHeader)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	params := make([]string, len(cols))
	quoted := make([]string, len(cols))
	for i, c := range cols {
		params[i], quoted[i] = d.Param(i+1), d.Quote(c)
	}
	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", qualified(d, req.Table), strings.Join(quoted, ", "), strings.Join(params, ", "))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // no-op tras Commit
	prep, err := tx.PrepareContext(ctx, stmt)
	if err != nil {
		return nil, err
	}
	defer prep.Close()

	start := time.Now()
	res := &ImportResult{}
	args := make([]any, len(cols))
	lastEmit := time.Now()
	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		line := r.Line()
		if len(rec) <= maxSrc {
			return nil, fmt.Errorf("fila %d: tiene %d columna(s) y el mapeo necesita al menos %d; no se importó nada", line, len(rec), maxSrc+1)
		}
		for i, c := range cols {
			v := rec[srcs[i]]
			if v == "" && (req.EmptyAsNull || !textualType.MatchString(types[c])) {
				args[i] = nil // una cadena vacía no es un número ni una fecha válidos: se toma como NULL
			} else {
				args[i] = v
			}
		}
		if _, err := prep.ExecContext(ctx, args...); err != nil {
			if ctx.Err() != nil {
				res.Cancelled = true
				return res, nil
			}
			return nil, fmt.Errorf("fila %d: %w (no se importó nada)", line, err)
		}
		res.Inserted++
		if res.Inserted%500 == 0 && time.Since(lastEmit) > 250*time.Millisecond {
			s.events.Emit("import:progress", map[string]any{"id": id, "rows": res.Inserted})
			lastEmit = time.Now()
		}
	}
	if ctx.Err() != nil {
		res.Cancelled = true
		return res, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}
