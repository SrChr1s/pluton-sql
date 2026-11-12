package service

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SrChr1s/pluton-sql/internal/driver"
	"github.com/SrChr1s/pluton-sql/internal/transfer"
)

// ScriptObject identifica un objeto de la base que se puede incluir en un script.
type ScriptObject struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // table | view | procedure | function
	// Args e ID solo los usan las rutinas (ver driver.RoutineInfo).
	Args string `json:"args"`
	ID   string `json:"id"`
}

func (o ScriptObject) routine() driver.RoutineInfo {
	return driver.RoutineInfo{Name: o.Name, Schema: o.Schema, Type: o.Kind, Args: o.Args, ID: o.ID}
}

// ScriptOptions describe qué incluye el script generado.
type ScriptOptions struct {
	Objects        []ScriptObject `json:"objects"` // vacío = todos los de la base
	Mode           string         `json:"mode"`    // schema | data | both
	IncludeDrop    bool           `json:"includeDrop"`
	IncludeSchemas bool           `json:"includeSchemas"` // CREATE SCHEMA
	IncludeIndexes bool           `json:"includeIndexes"`
	IncludeFKs     bool           `json:"includeForeignKeys"`
	BatchRows      int            `json:"batchRows"` // filas por INSERT

	maxRows int  // límite de filas por tabla (0 = todas); lo usa "Script como → Datos"
	bare    bool // sin cabecera, prólogo ni epílogo
}

// ScriptProgress se emite mientras se genera el script.
type ScriptProgress struct {
	ID     string `json:"id"`
	Phase  string `json:"phase"` // schema | data
	Object string `json:"object"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Rows   int64  `json:"rows"`
}

// ScriptStats resume lo que se escribió. Errors lista los objetos que no se pudieron escribir
// (el script lleva en su lugar un comentario "-- ERROR").
type ScriptStats struct {
	Objects int      `json:"objects"`
	Rows    int64    `json:"rows"`
	Errors  []string `json:"errors"`
}

// SQL Server admite como mucho 1000 filas en un VALUES.
const maxInsertRows = 1000

// listScriptObjects lista tablas, vistas y rutinas de todos los esquemas de la base conectada.
func listScriptObjects(ctx context.Context, db *sql.DB, d driver.Dialect) ([]ScriptObject, error) {
	schemas, err := d.ListSchemas(ctx, db)
	if err != nil {
		return nil, err
	}
	out := []ScriptObject{}
	for _, s := range schemas {
		tables, err := d.ListTables(ctx, db, s)
		if err != nil {
			return nil, err
		}
		for _, t := range tables {
			out = append(out, ScriptObject{Schema: s, Name: t.Name, Kind: t.Type})
		}
		// Las rutinas son de mejor esfuerzo: sin permiso sobre el catálogo el script sale sin ellas.
		if rs, err := d.ListRoutines(ctx, db, s); err == nil {
			for _, r := range rs {
				out = append(out, ScriptObject{Schema: s, Name: r.Name, Kind: r.Type, Args: r.Args, ID: r.ID})
			}
		}
	}
	return out, nil
}

type scriptTable struct {
	obj  ScriptObject
	meta []driver.ColumnInfo
	ts   driver.TableScript
	fks  []driver.ForeignKeyInfo
	err  error
}

type scriptGen struct {
	ctx      context.Context
	db       *sql.DB
	d        driver.Dialect
	engine   string
	database string
	opt      ScriptOptions
	w        io.Writer
	progress func(ScriptProgress)
	stats    ScriptStats
	werr     error // primer error de escritura: aborta todo
}

// generateScript escribe en w un script que recrea los objetos pedidos, en un orden ejecutable:
// esquemas, tablas (padres antes que hijas), datos, índices, claves foráneas, vistas y rutinas.
// Un objeto que falla deja un comentario y no detiene el resto; un error de escritura o una
// cancelación sí.
func generateScript(ctx context.Context, db *sql.DB, d driver.Dialect, database string, opt ScriptOptions, w io.Writer, progress func(ScriptProgress)) (*ScriptStats, error) {
	switch opt.Mode {
	case "":
		opt.Mode = "both"
	case "schema", "data", "both":
	default:
		return nil, fmt.Errorf("modo de script no soportado: %q", opt.Mode)
	}
	if opt.BatchRows <= 0 {
		opt.BatchRows = 100
	}
	opt.BatchRows = min(opt.BatchRows, maxInsertRows)
	if progress == nil {
		progress = func(ScriptProgress) {}
	}
	g := &scriptGen{ctx: ctx, db: db, d: d, engine: d.Name(), database: database, opt: opt, w: w, progress: progress, stats: ScriptStats{Errors: []string{}}}
	schemaOn, dataOn := opt.Mode != "data", opt.Mode != "schema"

	objs := opt.Objects
	if len(objs) == 0 {
		var err error
		if objs, err = listScriptObjects(ctx, db, d); err != nil {
			return nil, err
		}
	}
	tables, views, routines := splitObjects(objs)
	if !schemaOn {
		views, routines = nil, nil
	}
	total := len(tables) + len(views) + len(routines)

	// Metadatos de las tablas y orden por claves foráneas.
	ts := make([]*scriptTable, len(tables))
	for i, o := range tables {
		ts[i] = g.loadTable(o)
	}
	ts = sortTables(ts)

	if !opt.bare {
		g.header()
	}

	// DROP en orden inverso al de creación.
	if schemaOn && opt.IncludeDrop {
		for i := len(routines) - 1; i >= 0; i-- {
			g.emit(dropStmt(g.d, routines[i]))
		}
		for i := len(views) - 1; i >= 0; i-- {
			g.emit(dropStmt(g.d, views[i]))
		}
		for i := len(ts) - 1; i >= 0; i-- {
			g.emit(dropStmt(g.d, ts[i].obj))
		}
	}

	if schemaOn && opt.IncludeSchemas {
		g.createSchemas(tables, views, routines)
	}

	done := 0
	step := func(phase string, o ScriptObject, n int) {
		g.progress(ScriptProgress{Phase: phase, Object: qualifiedName(o), Done: n, Total: total})
	}
	if schemaOn {
		for _, t := range ts {
			step("schema", t.obj, done)
			done++
			g.comment("Tabla", t.obj)
			if t.err != nil {
				g.fail(t.obj, t.err)
				continue
			}
			g.emit(t.ts.Create)
			for _, c := range t.ts.Comments {
				g.emit(c)
			}
			g.stats.Objects++
		}
	}
	if g.werr != nil {
		return nil, g.werr
	}

	if dataOn {
		for i, t := range ts {
			if t.err != nil {
				if !schemaOn { // en modo esquema el error ya se anotó
					g.comment("Datos de", t.obj)
					g.fail(t.obj, t.err)
				}
				continue
			}
			g.progress(ScriptProgress{Phase: "data", Object: qualifiedName(t.obj), Done: i, Total: len(ts), Rows: g.stats.Rows})
			if err := g.writeData(t, i, len(ts)); err != nil {
				return nil, err
			}
			if !schemaOn {
				g.stats.Objects++
			}
		}
	}
	if err := g.alive(); err != nil {
		return nil, err
	}

	if schemaOn {
		if opt.IncludeIndexes {
			for _, t := range ts {
				for _, ix := range t.ts.Indexes {
					g.emit(ix)
				}
			}
		}
		if opt.IncludeFKs {
			for _, t := range ts {
				for _, fk := range t.ts.ForeignKeys {
					g.emit(fk)
				}
			}
		}
		for _, v := range g.sortViews(views) {
			step("schema", v.obj, done)
			done++
			g.comment("Vista", v.obj)
			if v.err != nil {
				g.fail(v.obj, v.err)
				continue
			}
			g.emit(v.ddl)
			g.stats.Objects++
		}
		// Funciones antes que procedimientos: un procedimiento puede llamarlas.
		sort.SliceStable(routines, func(i, j int) bool { return routines[i].Kind == "function" && routines[j].Kind != "function" })
		for _, r := range routines {
			step("schema", r, done)
			done++
			g.comment(map[bool]string{true: "Función", false: "Procedimiento"}[r.Kind == "function"], r)
			def, err := g.d.RoutineDDL(g.ctx, g.db, r.Schema, r.routine())
			if err != nil {
				g.fail(r, err)
				continue
			}
			g.emitRoutine(def)
			g.stats.Objects++
		}
	}
	if !opt.bare {
		g.footer()
	}
	if g.werr != nil {
		return nil, g.werr
	}
	return &g.stats, g.alive()
}

func splitObjects(objs []ScriptObject) (tables, views, routines []ScriptObject) {
	seen := map[ScriptObject]bool{}
	for _, o := range objs {
		if seen[o] {
			continue
		}
		seen[o] = true
		switch o.Kind {
		case "table":
			tables = append(tables, o)
		case "view":
			views = append(views, o)
		case "procedure", "function":
			routines = append(routines, o)
		}
	}
	byName := func(s []ScriptObject) {
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Schema != s[j].Schema {
				return s[i].Schema < s[j].Schema
			}
			return s[i].Name < s[j].Name
		})
	}
	byName(tables)
	byName(views)
	byName(routines)
	return
}

func qualifiedName(o ScriptObject) string {
	if o.Schema == "" {
		return o.Name
	}
	return o.Schema + "." + o.Name
}

func (g *scriptGen) quoted(o ScriptObject) string {
	return qualified(g.d, TableRef{Schema: o.Schema, Table: o.Name})
}

// alive devuelve el error del contexto si la generación se canceló o venció.
func (g *scriptGen) alive() error {
	if g.werr != nil {
		return g.werr
	}
	return g.ctx.Err()
}

func (g *scriptGen) write(s string) {
	if g.werr != nil {
		return
	}
	if _, err := io.WriteString(g.w, s); err != nil {
		g.werr = err
	}
}

// emit escribe una sentencia con el terminador del motor: GO en SQL Server, ";" en el resto.
func (g *scriptGen) emit(stmt string) {
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return
	}
	if g.engine == "mssql" {
		g.write(stmt + "\nGO\n\n")
		return
	}
	if !strings.HasSuffix(stmt, ";") {
		stmt += ";"
	}
	g.write(stmt + "\n\n")
}

// MySQL no admite el cuerpo de una rutina (que lleva ";") sin cambiar el delimitador del cliente.
func (g *scriptGen) emitRoutine(def string) {
	if g.engine == "mysql" {
		g.write("DELIMITER $$\n" + strings.TrimRight(strings.TrimSpace(def), ";") + "$$\nDELIMITER ;\n\n")
		return
	}
	g.emit(def)
}

var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ")

func (g *scriptGen) comment(kind string, o ScriptObject) {
	g.write("-- " + kind + " " + lineBreaks.Replace(g.quoted(o)) + "\n")
}

// fail anota en el script que un objeto no se pudo escribir y lo cuenta en el resumen.
func (g *scriptGen) fail(o ScriptObject, err error) {
	msg := fmt.Sprintf("%s: %v", qualifiedName(o), err)
	g.stats.Errors = append(g.stats.Errors, msg)
	g.write("-- ERROR: " + lineBreaks.Replace(msg) + "\n\n")
}

func (g *scriptGen) header() {
	modes := map[string]string{"schema": "esquema", "data": "datos", "both": "esquema y datos"}
	g.write("-- Script generado por Pluton SQL\n")
	g.write("-- Base de datos: " + lineBreaks.Replace(g.database) + " (" + g.engine + ")\n")
	g.write("-- Contenido: " + modes[g.opt.Mode] + "\n")
	g.write("-- Fecha: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")
	switch g.engine {
	case "mssql":
		g.emit("SET ANSI_NULLS ON")
		g.emit("SET QUOTED_IDENTIFIER ON")
	case "mysql":
		g.emit("SET NAMES utf8mb4")
		g.emit("SET FOREIGN_KEY_CHECKS=0")
	case "sqlite":
		g.emit("PRAGMA foreign_keys=OFF")
	}
}

func (g *scriptGen) footer() {
	switch g.engine {
	case "mysql":
		g.emit("SET FOREIGN_KEY_CHECKS=1")
	case "sqlite":
		g.emit("PRAGMA foreign_keys=ON")
	}
}

func (g *scriptGen) createSchemas(groups ...[]ScriptObject) {
	implicit := map[string]bool{"": true, "dbo": g.engine == "mssql", "public": g.engine == "postgres"}
	seen := map[string]bool{}
	var names []string
	for _, grp := range groups {
		for _, o := range grp {
			if !implicit[o.Schema] && !seen[o.Schema] {
				seen[o.Schema] = true
				names = append(names, o.Schema)
			}
		}
	}
	sort.Strings(names)
	for _, s := range names {
		switch g.engine {
		case "mssql":
			g.emit(fmt.Sprintf("IF SCHEMA_ID(N'%s') IS NULL EXEC(N'CREATE SCHEMA %s')", strings.ReplaceAll(s, "'", "''"), strings.ReplaceAll(g.d.Quote(s), "'", "''")))
		case "postgres":
			g.emit("CREATE SCHEMA IF NOT EXISTS " + g.d.Quote(s))
		}
	}
}

// dropStmt es el DROP ... IF EXISTS de un objeto.
func dropStmt(d driver.Dialect, o ScriptObject) string {
	qn := qualified(d, TableRef{Schema: o.Schema, Table: o.Name})
	kind := map[string]string{"table": "TABLE", "view": "VIEW", "procedure": "PROCEDURE", "function": "FUNCTION"}[o.Kind]
	if d.Name() == "mssql" {
		// DROP ... IF EXISTS solo existe desde SQL Server 2016.
		code := map[string]string{"table": ", N'U'", "view": ", N'V'", "procedure": ", N'P'"}[o.Kind]
		return fmt.Sprintf("IF OBJECT_ID(N'%s'%s) IS NOT NULL DROP %s %s", strings.ReplaceAll(qn, "'", "''"), code, kind, qn)
	}
	s := "DROP " + kind + " IF EXISTS " + qn
	switch {
	case d.Name() == "postgres" && o.Kind == "table":
		s += " CASCADE"
	case d.Name() == "postgres" && (o.Kind == "procedure" || o.Kind == "function"):
		s += "(" + o.Args + ")"
	}
	return s
}

// loadTable reúne columnas, piezas del DDL y claves foráneas de una tabla.
func (g *scriptGen) loadTable(o ScriptObject) *scriptTable {
	t := &scriptTable{obj: o}
	if t.meta, t.err = g.d.ListColumns(g.ctx, g.db, o.Schema, o.Name); t.err != nil {
		return t
	}
	if t.ts, t.err = g.d.TableScript(g.ctx, g.db, o.Schema, o.Name); t.err != nil {
		return t
	}
	t.fks, _ = g.d.ListForeignKeys(g.ctx, g.db, o.Schema, o.Name) // solo para ordenar
	return t
}

// sortTables ordena las tablas para que cada una vaya después de las que referencia por clave foránea.
// Las referencias circulares o a tablas fuera del script no cuentan: las claves se crean al final.
func sortTables(ts []*scriptTable) []*scriptTable {
	idx := map[string]int{}
	for i, t := range ts {
		idx[t.obj.Schema+"\x00"+t.obj.Name] = i
	}
	deps := make([][]int, len(ts))
	for i, t := range ts {
		for _, fk := range t.fks {
			schema := fk.RefSchema
			if t.obj.Schema == "" { // MySQL y SQLite no tienen esquema
				schema = ""
			}
			if j, ok := idx[schema+"\x00"+fk.RefTable]; ok && j != i {
				deps[i] = append(deps[i], j)
			}
		}
	}
	order := topoOrder(len(ts), func(i int) []int { return deps[i] })
	out := make([]*scriptTable, len(ts))
	for k, i := range order {
		out[k] = ts[i]
	}
	return out
}

// topoOrder devuelve los índices 0..n-1 con cada nodo después de sus dependencias; a igualdad respeta el
// orden original. Si hay un ciclo, los nodos implicados se añaden al final en su orden original.
func topoOrder(n int, deps func(int) []int) []int {
	state := make([]int, n) // 0 = pendiente, 1 = en curso, 2 = hecho
	out := make([]int, 0, n)
	var visit func(int)
	visit = func(i int) {
		if state[i] != 0 {
			return // hecho, o ciclo: se corta aquí
		}
		state[i] = 1
		for _, j := range deps(i) {
			visit(j)
		}
		state[i] = 2
		out = append(out, i)
	}
	for i := 0; i < n; i++ {
		visit(i)
	}
	return out
}

type scriptView struct {
	obj ScriptObject
	ddl string
	err error
}

// sortViews lee la definición de cada vista y las ordena de modo que una vista que menciona a otra vaya
// después de ella (heurística por nombre; no hace falta que sea exacta, solo evita los casos habituales).
func (g *scriptGen) sortViews(views []ScriptObject) []scriptView {
	vs := make([]scriptView, len(views))
	for i, o := range views {
		vs[i] = scriptView{obj: o}
		vs[i].ddl, vs[i].err = g.d.TableDDL(g.ctx, g.db, o.Schema, o.Name, true)
	}
	pats := make([]*regexp.Regexp, len(vs))
	for i, v := range vs {
		pats[i] = regexp.MustCompile(`(?i)(^|[^\w])` + regexp.QuoteMeta(v.obj.Name) + `($|[^\w])`)
	}
	order := topoOrder(len(vs), func(i int) []int {
		var d []int
		for j := range vs {
			if j != i && vs[i].err == nil && pats[j].MatchString(vs[i].ddl) {
				d = append(d, j)
			}
		}
		return d
	})
	out := make([]scriptView, len(vs))
	for k, i := range order {
		out[k] = vs[i]
	}
	return out
}

// writeData escribe los INSERT de una tabla. Un error de la consulta se anota y se sigue; solo un error
// de escritura o la cancelación devuelven error.
func (g *scriptGen) writeData(t *scriptTable, n, total int) error {
	skip := map[string]bool{}
	for _, c := range t.ts.SkipColumns {
		skip[c] = true
	}
	var names, exprs []string
	for _, c := range t.meta {
		if skip[c.Name] {
			continue
		}
		names = append(names, c.Name)
		q := g.d.Quote(c.Name)
		// El driver de SQL Server entrega los uniqueidentifier como bytes, que no se pueden escribir como texto.
		if g.engine == "mssql" && strings.EqualFold(c.Type, "uniqueidentifier") {
			q = "CAST(" + q + " AS varchar(36)) AS " + q
		}
		exprs = append(exprs, q)
	}
	g.comment("Datos de", t.obj)
	if len(names) == 0 {
		g.write("\n")
		return nil
	}
	qn := g.quoted(t.obj)
	keys, hasPK := keyColumns(t.meta, g.engine)
	order, _ := orderClause(g.d, t.meta, "", false, keys, hasPK)
	list := strings.Join(exprs, ", ")
	query, hidden := "SELECT "+list+" FROM "+qn, 0
	if order != "" {
		query += " " + order
	}
	if g.opt.maxRows > 0 {
		query, hidden = g.d.PageSQL("(SELECT "+list+" FROM "+qn+") AS __t", "", order, g.opt.maxRows, 0)
	}

	rows, err := g.db.QueryContext(g.ctx, query)
	if err != nil {
		g.write("\n")
		g.fail(t.obj, err)
		return g.alive()
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		g.write("\n")
		g.fail(t.obj, err)
		return g.alive()
	}

	var so transfer.SQLOptions
	switch g.engine {
	case "mssql":
		so.Sep = "GO"
		if t.ts.Identity {
			so.Before, so.After = "SET IDENTITY_INSERT "+qn+" ON;", "SET IDENTITY_INSERT "+qn+" OFF;"
		}
	case "postgres":
		if t.ts.Identity {
			so.Override = "OVERRIDING SYSTEM VALUE"
		}
	}
	w := transfer.NewSQLWith(g.w, g.d, qn, g.opt.BatchRows, so)
	if err := w.Begin(names); err != nil {
		return err
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	last := time.Now()
	var scanErr error
	for rows.Next() {
		if scanErr = rows.Scan(ptrs...); scanErr != nil {
			break
		}
		if err := w.Row(vals[hidden:]); err != nil {
			return err
		}
		g.stats.Rows++
		if g.stats.Rows%1000 == 0 && time.Since(last) > 250*time.Millisecond {
			g.progress(ScriptProgress{Phase: "data", Object: qualifiedName(t.obj), Done: n, Total: total, Rows: g.stats.Rows})
			last = time.Now()
		}
	}
	if scanErr == nil {
		scanErr = rows.Err()
	}
	if err := w.End(); err != nil { // vuelca las filas pendientes, también tras un error de lectura
		return err
	}
	g.write("\n")
	if scanErr != nil {
		if err := g.alive(); err != nil {
			return err
		}
		g.fail(t.obj, scanErr)
		return nil
	}

	// Tras insertar valores explícitos, las secuencias de PostgreSQL siguen donde estaban.
	if g.engine == "postgres" && !g.opt.bare {
		for _, c := range t.ts.SeqColumns {
			g.emit(fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE(MAX(%s), 1), MAX(%s) IS NOT NULL) FROM %s",
				strings.ReplaceAll(qn, "'", "''"), strings.ReplaceAll(c, "'", "''"), g.d.Quote(c), g.d.Quote(c), qn))
		}
	}
	return g.alive()
}
