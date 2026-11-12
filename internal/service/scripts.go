package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxEditorScript es lo máximo que se devuelve para abrirlo en el editor; por encima hay que guardarlo en un archivo.
const maxEditorScript = 8 << 20

// ScriptRequest pide un script de la base. Con Path lo guarda en ese archivo; sin él lo devuelve en Script.
type ScriptRequest struct {
	Profile  string        `json:"profile"`
	Database string        `json:"database"`
	Options  ScriptOptions `json:"options"`
	Path     string        `json:"path"`
}

type ScriptResult struct {
	Path       string   `json:"path"`
	Script     string   `json:"script"`
	Objects    int      `json:"objects"`
	Rows       int64    `json:"rows"`
	Bytes      int64    `json:"bytes"`
	DurationMs int64    `json:"durationMs"`
	Cancelled  bool     `json:"cancelled"`
	Errors     []string `json:"errors"`
}

// ObjectScriptRequest pide el script de un solo objeto ("Script como…").
type ObjectScriptRequest struct {
	Profile  string       `json:"profile"`
	Database string       `json:"database"`
	Object   ScriptObject `json:"object"`
	Action   string       `json:"action"` // create | drop | select | insert | data
}

type ScriptService struct {
	pools  Pools
	events *Events
	jobs   jobs
}

func NewScriptService(pools Pools, events *Events) *ScriptService {
	return &ScriptService{pools: pools, events: events}
}

// Cancel interrumpe una generación en curso.
func (s *ScriptService) Cancel(id string) { s.jobs.cancel(id) }

// Objects lista lo que se puede incluir en un script: tablas, vistas, procedimientos y funciones.
func (s *ScriptService) Objects(profile, database string) ([]ScriptObject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	db, d, err := s.pools.Get(ctx, profile, database)
	if err != nil {
		return nil, err
	}
	return listScriptObjects(ctx, db, d)
}

// capWriter falla al pasar de max bytes: evita montar en memoria un script gigante para el editor.
type capWriter struct {
	buf bytes.Buffer
	max int
}

var errTooLarge = errors.New("el script es demasiado grande para abrirlo en el editor; guárdalo en un archivo")

func (c *capWriter) Write(p []byte) (int, error) {
	if c.buf.Len()+len(p) > c.max {
		return 0, errTooLarge
	}
	return c.buf.Write(p)
}

// Generate genera el script. El archivo de destino solo se reemplaza si la generación termina entera.
func (s *ScriptService) Generate(id string, req ScriptRequest) (*ScriptResult, error) {
	if req.Path != "" {
		if st, err := os.Stat(filepath.Dir(req.Path)); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("la carpeta de destino no existe: %s", filepath.Dir(req.Path))
		}
	}
	ctx, done := s.jobs.start(id)
	defer done()

	db, d, err := s.pools.Get(ctx, req.Profile, req.Database)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	res := &ScriptResult{Path: req.Path, Errors: []string{}}
	progress := func(p ScriptProgress) {
		p.ID = id
		s.events.Emit("script:progress", p)
	}

	var stats *ScriptStats
	if req.Path == "" {
		cw := &capWriter{max: maxEditorScript}
		if stats, err = generateScript(ctx, db, d, req.Database, req.Options, cw, progress); err == nil {
			res.Script = cw.buf.String()
		}
	} else {
		part := req.Path + ".part"
		f, ferr := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if ferr != nil {
			return nil, ferr
		}
		ok := false
		defer func() {
			if !ok {
				f.Close()
				os.Remove(part) // ni cancelar ni fallar deja un archivo a medias
			}
		}()
		bw := bufio.NewWriterSize(f, 256<<10)
		if stats, err = generateScript(ctx, db, d, req.Database, req.Options, bw, progress); err == nil {
			if err = bw.Flush(); err == nil {
				if err = f.Close(); err == nil {
					if err = os.Rename(part, req.Path); err == nil {
						ok = true
						if st, serr := os.Stat(req.Path); serr == nil {
							res.Bytes = st.Size()
						}
					}
				}
			}
		}
	}
	if err != nil {
		// Un error que llega con el contexto cancelado es consecuencia de la cancelación, no un fallo.
		if ctx.Err() != nil && !errors.Is(err, errTooLarge) {
			res.Cancelled = true
			return res, nil
		}
		return nil, err
	}
	res.Objects, res.Rows, res.Errors = stats.Objects, stats.Rows, stats.Errors
	if req.Path == "" {
		res.Bytes = int64(len(res.Script))
	}
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

// ObjectScript devuelve el script de un objeto: CREATE, DROP, SELECT, una plantilla de INSERT o sus datos.
func (s *ScriptService) ObjectScript(req ObjectScriptRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dataTimeout)
	defer cancel()
	db, d, err := s.pools.Get(ctx, req.Profile, req.Database)
	if err != nil {
		return "", err
	}
	o := req.Object
	isRoutine := o.Kind == "procedure" || o.Kind == "function"
	isTable := o.Kind == "table"
	if !isRoutine && !isTable && o.Kind != "view" {
		return "", fmt.Errorf("tipo de objeto no soportado: %q", o.Kind)
	}

	switch req.Action {
	case "create":
		if isRoutine {
			return d.RoutineDDL(ctx, db, o.Schema, o.routine())
		}
		return d.TableDDL(ctx, db, o.Schema, o.Name, o.Kind == "view")
	case "drop":
		return dropStmt(d, o) + ";", nil
	case "select", "insert":
		if isRoutine || (req.Action == "insert" && !isTable) {
			return "", fmt.Errorf("%q no aplica a este tipo de objeto", req.Action)
		}
		cols, err := d.ListColumns(ctx, db, o.Schema, o.Name)
		if err != nil {
			return "", err
		}
		qn := qualified(d, TableRef{Schema: o.Schema, Table: o.Name})
		var names, holes []string
		for _, c := range cols {
			if req.Action == "insert" && c.AutoIncrement {
				continue // lo genera el motor
			}
			names = append(names, d.Quote(c.Name))
			holes = append(holes, "NULL /* "+strings.ReplaceAll(c.Type, "*/", "* /")+" */")
		}
		if len(names) == 0 {
			return "", errors.New("la tabla no tiene columnas que insertar")
		}
		if req.Action == "select" {
			return "SELECT\n    " + strings.Join(names, ",\n    ") + "\nFROM " + qn + ";", nil
		}
		return "INSERT INTO " + qn + "\n    (" + strings.Join(names, ", ") + ")\nVALUES\n    (" + strings.Join(holes, ", ") + ");", nil
	case "data":
		if !isTable {
			return "", errors.New("solo las tablas tienen datos que escribir")
		}
		var sb strings.Builder
		opt := ScriptOptions{Objects: []ScriptObject{o}, Mode: "data", bare: true, maxRows: 1000}
		if _, err := generateScript(ctx, db, d, req.Database, opt, &sb, nil); err != nil {
			return "", err
		}
		return strings.TrimSpace(sb.String()) + "\n", nil
	}
	return "", fmt.Errorf("acción no soportada: %q", req.Action)
}
