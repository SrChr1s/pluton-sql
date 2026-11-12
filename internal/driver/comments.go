package driver

import (
	"fmt"
	"strings"
)

// MaxCommentLen es la longitud máxima de un comentario de columna: el límite de MySQL, el más estricto.
const MaxCommentLen = 1024

// StrLit escribe s como literal de cadena estándar ('…', con las comillas duplicadas). Vale para PostgreSQL
// (con standard_conforming_strings, que viene activo) y para SQLite.
func StrLit(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// NStrLit escribe s como literal Unicode de SQL Server (N'…').
func NStrLit(s string) string { return "N" + StrLit(s) }

// MySQLStrLit escribe s como literal de MySQL, que además interpreta las barras invertidas.
func MySQLStrLit(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, "'", "''", "\x00", `\0`).Replace(s) + "'"
}

// ColumnCommentSQL devuelve las sentencias que dejan comment como comentario de la columna en PostgreSQL y
// SQL Server (en MySQL el comentario va dentro de la definición de la columna, y SQLite no tiene).
//
// replace=false genera un comentario nuevo (para scripts: el llamador omite los vacíos). replace=true es para
// cambiar uno que puede existir: en PostgreSQL un comentario vacío lo quita (IS NULL), y en SQL Server se borra
// la propiedad si existe antes de añadir la nueva, así no hay que saber si ya estaba.
func ColumnCommentSQL(engine, schema, table, column, comment string, replace bool) []string {
	comment = strings.TrimSpace(comment)
	switch engine {
	case "postgres":
		qn := quoteWith(`"`, `"`, schema) + "." + quoteWith(`"`, `"`, table) + "." + quoteWith(`"`, `"`, column)
		if comment == "" {
			if !replace {
				return nil
			}
			return []string{"COMMENT ON COLUMN " + qn + " IS NULL"}
		}
		return []string{"COMMENT ON COLUMN " + qn + " IS " + StrLit(comment)}
	case "mssql":
		target := fmt.Sprintf("N'SCHEMA', %s, N'TABLE', %s, N'COLUMN', %s", NStrLit(schema), NStrLit(table), NStrLit(column))
		var out []string
		if replace {
			out = append(out, fmt.Sprintf("IF EXISTS (SELECT 1 FROM sys.fn_listextendedproperty(N'MS_Description', %s)) EXEC sys.sp_dropextendedproperty N'MS_Description', %s", target, target))
		}
		if comment != "" {
			out = append(out, fmt.Sprintf("EXEC sys.sp_addextendedproperty N'MS_Description', %s, %s", NStrLit(comment), target))
		}
		return out
	}
	return nil
}
