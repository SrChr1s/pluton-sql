package service

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// DefaultRowLimit evita saturar el webview con resultados enormes.
const DefaultRowLimit = 1000

type QueryResult struct {
	Columns      []string `json:"columns"`
	Rows         [][]any  `json:"rows"`
	RowsAffected int64    `json:"rowsAffected"`
	Truncated    bool     `json:"truncated"`
	DurationMs   int64    `json:"durationMs"`
	IsQuery      bool     `json:"isQuery"`
}

// Recorder registra cada ejecución (lo implementa el historial). Es opcional.
type Recorder interface {
	Record(profile, database, sql string, durationMs, rows int64, isQuery bool, errMsg string)
}

type QueryService struct {
	pools   Pools
	rec     Recorder
	mu      sync.Mutex
	running map[string]context.CancelFunc
}

func NewQueryService(pools Pools, rec Recorder) *QueryService {
	return &QueryService{pools: pools, rec: rec, running: map[string]context.CancelFunc{}}
}

// Execute ejecuta sqlText. queryID lo elige el frontend para poder cancelar.
func (s *QueryService) Execute(queryID, profileID, database, sqlText string) (*QueryResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[queryID] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.running, queryID)
		s.mu.Unlock()
	}()

	db, d, err := s.pools.Get(ctx, profileID, database)
	if err != nil {
		return nil, err // no hubo ejecución: no se registra en el historial
	}
	start := time.Now()
	var res *QueryResult
	if d.Name() == "mssql" {
		res, err = s.runBatches(ctx, db, sqlText)
	} else {
		res, err = s.run(ctx, db, sqlText)
	}
	if res != nil {
		res.DurationMs = time.Since(start).Milliseconds()
	}
	if s.rec != nil {
		var rows int64
		isQuery := returnsRows(sqlText)
		msg := ""
		switch {
		case err != nil:
			msg = err.Error()
		case res != nil && res.IsQuery:
			rows = int64(len(res.Rows))
		case res != nil:
			rows = res.RowsAffected
		}
		s.rec.Record(profileID, database, sqlText, time.Since(start).Milliseconds(), rows, isQuery, msg)
	}
	return res, err
}

func (s *QueryService) run(ctx context.Context, db querier, sqlText string) (*QueryResult, error) {
	res := &QueryResult{Columns: []string{}, Rows: [][]any{}}
	if returnsRows(sqlText) {
		res.IsQuery = true
		rows, err := db.QueryContext(ctx, sqlText)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		if err := readRows(rows, res, DefaultRowLimit); err != nil {
			return nil, err
		}
	} else {
		r, err := db.ExecContext(ctx, sqlText)
		if err != nil {
			return nil, err
		}
		res.RowsAffected, _ = r.RowsAffected()
	}
	return res, nil
}

// querier es lo que comparten *sql.DB y *sql.Conn.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// runBatches ejecuta un script de SQL Server separado por líneas GO. Todos los lotes van por la misma
// conexión (un USE o un SET debe seguir vigente en el lote siguiente). Devuelve el resultado del último
// lote que produce filas; si ninguno lo hace, la suma de filas afectadas.
func (s *QueryService) runBatches(ctx context.Context, db *sql.DB, script string) (*QueryResult, error) {
	batches := splitGoBatches(script)
	if len(batches) <= 1 && !strings.Contains(strings.ToUpper(script), "GO") {
		return s.run(ctx, db, script)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close() // al devolverla al pool, el driver reinicia la sesión (vuelve a la base original)
	var last *QueryResult
	var affected int64
	for i, b := range batches {
		for n := 0; n < b.count; n++ {
			r, err := s.run(ctx, conn, b.sql)
			if err != nil {
				if len(batches) > 1 {
					err = fmt.Errorf("lote %d de %d: %w", i+1, len(batches), err)
				}
				return nil, err
			}
			if r.IsQuery {
				last = r
			} else {
				affected += r.RowsAffected
			}
		}
	}
	if last == nil {
		last = &QueryResult{Columns: []string{}, Rows: [][]any{}, RowsAffected: affected}
	}
	return last, nil
}

// Cancel interrumpe una consulta en curso.
func (s *QueryService) Cancel(queryID string) {
	s.mu.Lock()
	cancel := s.running[queryID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func readRows(rows *sql.Rows, res *QueryResult, limit int) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	res.Columns = cols
	for rows.Next() {
		if len(res.Rows) >= limit {
			res.Truncated = true
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range vals {
			vals[i] = normalize(v)
		}
		res.Rows = append(res.Rows, vals)
	}
	return rows.Err()
}

// blobPreview son los bytes de un valor binario que se envían a la interfaz; el resto se recorta.
const blobPreview = 64

// normalize convierte valores de BD a algo serializable a JSON sin pérdida.
func normalize(v any) any {
	switch x := v.(type) {
	case []byte:
		if utf8.Valid(x) {
			return string(x)
		}
		if len(x) > blobPreview { // una imagen entera en hex (varios MB) congela la rejilla
			return "0x" + hex.EncodeToString(x[:blobPreview]) + "… (" + itoa(int64(len(x))) + " bytes)"
		}
		return "0x" + hex.EncodeToString(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case int64:
		if x > 1<<53 || x < -(1<<53) { // JS pierde precisión
			return itoa(x)
		}
		return x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return itoaF(x)
		}
		return x
	default:
		return v
	}
}

// returnsRows decide por la primera palabra si la sentencia produce filas.
func returnsRows(q string) bool {
	q = strings.TrimLeftFunc(q, func(r rune) bool { return unicode.IsSpace(r) || r == '(' })
	for strings.HasPrefix(q, "--") || strings.HasPrefix(q, "/*") { // saltar comentarios iniciales
		if strings.HasPrefix(q, "--") {
			i := strings.IndexByte(q, '\n')
			if i < 0 {
				return false
			}
			q = q[i+1:]
		} else {
			i := strings.Index(q, "*/")
			if i < 0 {
				return false
			}
			q = q[i+2:]
		}
		q = strings.TrimLeftFunc(q, unicode.IsSpace)
	}
	w := q
	if i := strings.IndexFunc(q, func(r rune) bool { return !unicode.IsLetter(r) }); i >= 0 {
		w = q[:i]
	}
	switch strings.ToUpper(w) {
	case "SELECT", "WITH", "SHOW", "PRAGMA", "EXPLAIN", "VALUES", "DESCRIBE", "DESC", "TABLE":
		return true
	}
	return false
}
