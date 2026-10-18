package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/SrChr1s/pluton-sql/internal/driver"
)

// ColumnDef describe una columna para añadirla, modificarla o crear una tabla.
type ColumnDef struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Nullable      bool   `json:"nullable"`
	Default       string `json:"default"` // expresión SQL; vacío = sin valor por defecto
	PrimaryKey    bool   `json:"primaryKey"`
	AutoIncrement bool   `json:"autoIncrement"`
}

type IndexDef struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

type FKDef struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefSchema  string   `json:"refSchema"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnDelete   string   `json:"onDelete"`
	OnUpdate   string   `json:"onUpdate"`
}

// SchemaOp es una modificación de estructura. Kind: add_column | drop_column | rename_column |
// alter_column | create_index | drop_index | add_fk | drop_fk | rename_table | drop_table | create_table.
type SchemaOp struct {
	Kind    string      `json:"kind"`
	Target  string      `json:"target"`  // columna, índice o clave foránea existente
	NewName string      `json:"newName"` // rename_*
	Column  *ColumnDef  `json:"column"`  // add_column, alter_column
	Index   *IndexDef   `json:"index"`   // create_index
	FK      *FKDef      `json:"fk"`      // add_fk
	Columns []ColumnDef `json:"columns"` // create_table
}

// SchemaPlan es el SQL que ejecutaría una operación, con avisos para el usuario.
type SchemaPlan struct {
	Statements []string `json:"statements"`
	Warnings   []string `json:"warnings"`
	// Transactional: si es false (MySQL) cada sentencia se confirma sola y un fallo puede dejar la estructura a medias.
	Transactional bool `json:"transactional"`
	Destructive   bool `json:"destructive"`
}

var (
	typeRe      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_ ,()\[\].]*$`)
	fkActionSet = map[string]bool{"": true, "NO ACTION": true, "CASCADE": true, "SET NULL": true, "SET DEFAULT": true, "RESTRICT": true}
)

func validName(what, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("falta el nombre de %s", what)
	}
	if strings.ContainsAny(s, "\x00\n\r") {
		return fmt.Errorf("el nombre de %s contiene caracteres no válidos", what)
	}
	return nil
}

func validType(t string) error {
	if !typeRe.MatchString(strings.TrimSpace(t)) {
		return fmt.Errorf("tipo no válido: %q", t)
	}
	return nil
}

func validExpr(e string) error {
	if strings.ContainsAny(e, ";\n\r") || strings.Contains(e, "--") || strings.Contains(e, "/*") {
		return errors.New("el valor por defecto debe ser una sola expresión, sin ';' ni comentarios")
	}
	return nil
}

func validColumnDef(c ColumnDef, needType bool) error {
	if err := validName("la columna", c.Name); err != nil {
		return err
	}
	if needType {
		if err := validType(c.Type); err != nil {
			return err
		}
	}
	return validExpr(c.Default)
}

func quoteList(d driver.Dialect, cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = d.Quote(c)
	}
	return strings.Join(q, ", ")
}

func nullSQL(engine string, nullable bool) string {
	switch {
	case !nullable:
		return " NOT NULL"
	case engine == "mssql":
		return " NULL" // la nulabilidad por defecto de SQL Server depende de la sesión
	}
	return ""
}

func defaultSQL(def string) string {
	if strings.TrimSpace(def) == "" {
		return ""
	}
	return " DEFAULT " + strings.TrimSpace(def)
}

func lit(s string) string { return "N'" + strings.ReplaceAll(s, "'", "''") + "'" }

// mssqlDropDefault elimina la restricción DEFAULT de una columna, cuyo nombre autogenerado hay que buscar.
// EXEC() no admite llamadas a funciones en su argumento: el texto se construye antes en una variable.
func mssqlDropDefault(qn, col string) string {
	return fmt.Sprintf("DECLARE @df sysname = (SELECT d.name FROM sys.default_constraints d JOIN sys.columns c ON c.default_object_id = d.object_id "+
		"WHERE d.parent_object_id = OBJECT_ID(%s) AND c.name = %s); "+
		"IF @df IS NOT NULL BEGIN DECLARE @sql nvarchar(max) = N'ALTER TABLE ' + %s + N' DROP CONSTRAINT ' + QUOTENAME(@df); EXEC(@sql); END", lit(qn), lit(col), lit(qn))
}

// buildPlan genera el SQL de op para el motor d. cur son las columnas actuales (nil en create_table).
func buildPlan(d driver.Dialect, ref TableRef, op SchemaOp, cur []driver.ColumnInfo, isView bool) (*SchemaPlan, error) {
	e := d.Name()
	qn := qualified(d, ref)
	p := &SchemaPlan{Statements: []string{}, Warnings: []string{}, Transactional: e != "mysql"}
	add := func(s string) { p.Statements = append(p.Statements, s) }
	warn := func(s string) { p.Warnings = append(p.Warnings, s) }
	unsupported := func(what string) (*SchemaPlan, error) {
		return nil, fmt.Errorf("SQLite no permite %s con ALTER TABLE; hay que recrear la tabla", what)
	}
	if !p.Transactional {
		warn("MySQL confirma cada sentencia de estructura por separado: si una falla, las anteriores ya quedaron aplicadas.")
	}
	current := func(name string) (driver.ColumnInfo, bool) {
		for _, c := range cur {
			if c.Name == name {
				return c, true
			}
		}
		return driver.ColumnInfo{}, false
	}

	switch op.Kind {
	case "add_column":
		if op.Column == nil {
			return nil, errors.New("falta la definición de la columna")
		}
		c := *op.Column
		if err := validColumnDef(c, true); err != nil {
			return nil, err
		}
		if _, ok := current(c.Name); ok {
			return nil, fmt.Errorf("ya existe una columna %q", c.Name)
		}
		if e == "sqlite" && !c.Nullable && strings.TrimSpace(c.Default) == "" {
			return nil, errors.New("SQLite exige un valor por defecto para añadir una columna NOT NULL")
		}
		kw := "ADD COLUMN "
		if e == "mssql" {
			kw = "ADD " // T-SQL no lleva la palabra COLUMN
		}
		add(fmt.Sprintf("ALTER TABLE %s %s%s %s%s%s", qn, kw, d.Quote(c.Name), strings.TrimSpace(c.Type), nullSQL(e, c.Nullable), defaultSQL(c.Default)))

	case "drop_column":
		if _, ok := current(op.Target); !ok {
			return nil, fmt.Errorf("la columna %q no existe", op.Target)
		}
		if len(cur) == 1 {
			return nil, errors.New("no se puede borrar la única columna de la tabla")
		}
		p.Destructive = true
		warn(fmt.Sprintf("Se borrarán la columna %q y todos sus datos.", op.Target))
		if e == "mssql" {
			add(mssqlDropDefault(qn, op.Target))
			warn("Si la columna forma parte de un índice o una clave, SQL Server exigirá eliminarlos antes.")
		}
		add(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", qn, d.Quote(op.Target)))

	case "rename_column":
		if _, ok := current(op.Target); !ok {
			return nil, fmt.Errorf("la columna %q no existe", op.Target)
		}
		if err := validName("la columna", op.NewName); err != nil {
			return nil, err
		}
		if _, ok := current(op.NewName); ok {
			return nil, fmt.Errorf("ya existe una columna %q", op.NewName)
		}
		if e == "mssql" {
			add(fmt.Sprintf("EXEC sp_rename %s, %s, N'COLUMN'", lit(qn+"."+d.Quote(op.Target)), lit(op.NewName)))
			warn("sp_rename no actualiza vistas, procedimientos ni otros objetos que usen la columna.")
		} else {
			add(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", qn, d.Quote(op.Target), d.Quote(op.NewName)))
		}

	case "alter_column":
		if e == "sqlite" {
			return unsupported("modificar el tipo, la nulabilidad o el valor por defecto de una columna")
		}
		if op.Column == nil {
			return nil, errors.New("falta la definición de la columna")
		}
		nw := *op.Column
		old, ok := current(op.Target)
		if !ok {
			return nil, fmt.Errorf("la columna %q no existe", op.Target)
		}
		nw.Name = op.Target // renombrar es otra operación
		if err := validColumnDef(nw, true); err != nil {
			return nil, err
		}
		typeChanged := !strings.EqualFold(strings.TrimSpace(nw.Type), strings.TrimSpace(old.Type))
		nullChanged := nw.Nullable != old.Nullable
		defChanged := strings.TrimSpace(nw.Default) != strings.TrimSpace(old.Default)
		if !typeChanged && !nullChanged && !defChanged {
			return nil, errors.New("no hay ningún cambio que aplicar")
		}
		if typeChanged {
			warn("Cambiar el tipo puede fallar o perder datos si hay valores que no encajan en el nuevo tipo.")
		}
		if nullChanged && !nw.Nullable {
			warn("Fallará si la columna ya contiene NULL.")
		}
		col := d.Quote(op.Target)
		switch e {
		case "postgres":
			if typeChanged {
				add(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s", qn, col, strings.TrimSpace(nw.Type), col, strings.TrimSpace(nw.Type)))
			}
			if nullChanged {
				act := "DROP NOT NULL"
				if !nw.Nullable {
					act = "SET NOT NULL"
				}
				add(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s %s", qn, col, act))
			}
			if defChanged {
				if strings.TrimSpace(nw.Default) == "" {
					add(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT", qn, col))
				} else {
					add(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s", qn, col, strings.TrimSpace(nw.Default)))
				}
			}
		case "mysql":
			auto := ""
			if old.AutoIncrement {
				auto = " AUTO_INCREMENT"
			}
			warn("MODIFY reescribe la definición completa de la columna: se conserva AUTO_INCREMENT, pero no comentarios ni collation.")
			add(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s%s%s%s", qn, col, strings.TrimSpace(nw.Type), nullSQL(e, nw.Nullable), defaultSQL(nw.Default), auto))
		case "mssql":
			if typeChanged || nullChanged {
				add(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s %s%s", qn, col, strings.TrimSpace(nw.Type), nullSQL(e, nw.Nullable)))
			}
			if defChanged {
				add(mssqlDropDefault(qn, op.Target))
				if strings.TrimSpace(nw.Default) != "" {
					add(fmt.Sprintf("ALTER TABLE %s ADD DEFAULT %s FOR %s", qn, strings.TrimSpace(nw.Default), col))
				}
			}
		}

	case "create_index":
		ix := op.Index
		if ix == nil || len(ix.Columns) == 0 {
			return nil, errors.New("elige al menos una columna para el índice")
		}
		if err := validName("el índice", ix.Name); err != nil {
			return nil, err
		}
		for _, c := range ix.Columns {
			if _, ok := current(c); !ok {
				return nil, fmt.Errorf("la columna %q no existe", c)
			}
		}
		u := ""
		if ix.Unique {
			u = "UNIQUE "
			warn("Un índice único fallará si ya hay valores repetidos en esas columnas.")
		}
		add(fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)", u, d.Quote(ix.Name), qn, quoteList(d, ix.Columns)))

	case "drop_index":
		if err := validName("el índice", op.Target); err != nil {
			return nil, err
		}
		switch e {
		case "postgres":
			add(fmt.Sprintf("DROP INDEX %s.%s", d.Quote(ref.Schema), d.Quote(op.Target)))
		case "sqlite":
			add("DROP INDEX " + d.Quote(op.Target))
		default:
			add(fmt.Sprintf("DROP INDEX %s ON %s", d.Quote(op.Target), qn))
		}

	case "add_fk":
		if e == "sqlite" {
			return unsupported("añadir claves foráneas")
		}
		fk := op.FK
		if fk == nil || len(fk.Columns) == 0 || len(fk.Columns) != len(fk.RefColumns) {
			return nil, errors.New("indica las columnas de origen y las referenciadas (el mismo número en ambas)")
		}
		if err := validName("la clave foránea", fk.Name); err != nil {
			return nil, err
		}
		if err := validName("la tabla referenciada", fk.RefTable); err != nil {
			return nil, err
		}
		if !fkActionSet[fk.OnDelete] || !fkActionSet[fk.OnUpdate] {
			return nil, errors.New("acción referencial no válida")
		}
		for _, c := range fk.Columns {
			if _, ok := current(c); !ok {
				return nil, fmt.Errorf("la columna %q no existe", c)
			}
		}
		rq := d.Quote(fk.RefTable)
		if fk.RefSchema != "" {
			rq = d.Quote(fk.RefSchema) + "." + rq
		}
		s := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", qn, d.Quote(fk.Name), quoteList(d, fk.Columns), rq, quoteList(d, fk.RefColumns))
		if fk.OnDelete != "" {
			s += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" {
			s += " ON UPDATE " + fk.OnUpdate
		}
		add(s)
		warn("Fallará si hay filas cuyo valor no existe en la tabla referenciada.")

	case "drop_fk":
		if e == "sqlite" {
			return unsupported("borrar claves foráneas")
		}
		if err := validName("la clave foránea", op.Target); err != nil {
			return nil, err
		}
		if e == "mysql" {
			add(fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", qn, d.Quote(op.Target)))
		} else {
			add(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", qn, d.Quote(op.Target)))
		}

	case "rename_table":
		if err := validName("la tabla", op.NewName); err != nil {
			return nil, err
		}
		if op.NewName == ref.Table {
			return nil, errors.New("el nombre nuevo es igual al actual")
		}
		if e == "sqlite" && isView {
			return nil, errors.New("SQLite no puede renombrar vistas")
		}
		switch e {
		case "mysql":
			add(fmt.Sprintf("RENAME TABLE %s TO %s", qn, d.Quote(op.NewName)))
		case "mssql":
			add(fmt.Sprintf("EXEC sp_rename %s, %s", lit(qn), lit(op.NewName)))
			warn("sp_rename no actualiza vistas, procedimientos ni otros objetos que usen la tabla.")
		default:
			add(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", qn, d.Quote(op.NewName)))
		}

	case "drop_table":
		p.Destructive = true
		kind := "TABLE"
		if isView {
			kind = "VIEW"
			warn("Se eliminará la vista; los datos de las tablas que consulta no se tocan.")
		} else {
			warn("Se borrarán la tabla y TODOS sus datos. No se puede deshacer.")
		}
		add(fmt.Sprintf("DROP %s %s", kind, qn))

	case "create_table":
		if err := validName("la tabla", ref.Table); err != nil {
			return nil, err
		}
		if len(op.Columns) == 0 {
			return nil, errors.New("añade al menos una columna")
		}
		var lines, pk []string
		seen := map[string]bool{}
		sqliteInlinePK := false
		for _, c := range op.Columns {
			if err := validColumnDef(c, true); err != nil {
				return nil, err
			}
			if seen[strings.ToLower(c.Name)] {
				return nil, fmt.Errorf("la columna %q está repetida", c.Name)
			}
			seen[strings.ToLower(c.Name)] = true
			typ := strings.TrimSpace(c.Type)
			l := d.Quote(c.Name) + " " + typ
			if c.PrimaryKey {
				pk = append(pk, c.Name)
			}
			if c.AutoIncrement {
				switch e {
				case "postgres":
					l += " GENERATED BY DEFAULT AS IDENTITY"
				case "mysql":
					l += " AUTO_INCREMENT"
				case "mssql":
					l += " IDENTITY(1,1)"
				case "sqlite":
					if !c.PrimaryKey || !strings.EqualFold(typ, "INTEGER") {
						return nil, errors.New("en SQLite el autoincremento exige una columna INTEGER que sea la clave primaria")
					}
					l += " PRIMARY KEY AUTOINCREMENT"
					sqliteInlinePK = true
				}
			}
			if c.PrimaryKey || !c.Nullable {
				l += " NOT NULL"
			} else {
				l += nullSQL(e, true)
			}
			l += defaultSQL(c.Default)
			lines = append(lines, "    "+l)
		}
		if len(pk) > 0 && !sqliteInlinePK {
			lines = append(lines, "    PRIMARY KEY ("+quoteList(d, pk)+")")
		}
		add("CREATE TABLE " + qn + " (\n" + strings.Join(lines, ",\n") + "\n)")

	default:
		return nil, fmt.Errorf("operación desconocida: %q", op.Kind)
	}
	return p, nil
}
