package service

import (
	"regexp"
	"strconv"
	"strings"
)

// batch es un lote de T-SQL; count > 1 cuando el separador fue "GO n".
type batch struct {
	sql   string
	count int
}

var goLine = regexp.MustCompile(`(?i)^\s*GO(?:\s+(\d+))?\s*(?:--.*)?$`)

// splitGoBatches parte un script T-SQL en lotes por las líneas "GO". GO no es T-SQL: lo entienden
// SSMS y sqlcmd, no el servidor, así que hay que separarlo antes de enviarlo. Una línea "GO" dentro
// de una cadena, un comentario de bloque o un identificador entre corchetes no es separador.
func splitGoBatches(script string) []batch {
	var out []batch
	var cur strings.Builder
	flush := func(count int) {
		if strings.TrimSpace(cur.String()) != "" {
			out = append(out, batch{sql: cur.String(), count: count})
		}
		cur.Reset()
	}

	depth := 0 // nivel de /* */ (T-SQL los anida)
	var q byte // ' " [ abierto, o 0
	for _, line := range strings.SplitAfter(script, "\n") {
		if depth == 0 && q == 0 {
			if m := goLine.FindStringSubmatch(strings.TrimRight(line, "\r\n")); m != nil {
				n := 1
				if m[1] != "" {
					n, _ = strconv.Atoi(m[1])
				}
				flush(max(n, 1))
				continue
			}
		}
		cur.WriteString(line)
		// Actualiza el estado léxico con esta línea.
		for i := 0; i < len(line); i++ {
			c := line[i]
			switch {
			case q != 0:
				end := map[byte]byte{'\'': '\'', '"': '"', '[': ']'}[q]
				if c == end {
					if i+1 < len(line) && line[i+1] == end {
						i++ // comilla o corchete duplicado: escape
					} else {
						q = 0
					}
				}
			case depth > 0:
				if c == '*' && i+1 < len(line) && line[i+1] == '/' {
					depth--
					i++
				} else if c == '/' && i+1 < len(line) && line[i+1] == '*' {
					depth++
					i++
				}
			case c == '-' && i+1 < len(line) && line[i+1] == '-':
				i = len(line) // comentario de línea
			case c == '/' && i+1 < len(line) && line[i+1] == '*':
				depth++
				i++
			case c == '\'' || c == '"' || c == '[':
				q = c
			}
		}
	}
	flush(1)
	return out
}
