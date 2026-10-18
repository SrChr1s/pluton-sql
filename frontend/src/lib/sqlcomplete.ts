// Autocompletado de SQL consciente del contexto: tablas tras FROM/JOIN, columnas tras "alias." y de las
// tablas de la consulta, con los nombres citados cuando el motor lo exige. Módulo puro: no depende del editor.
import { quoteIfNeeded, quoteIdent, unquoteIdent } from './sql'

export type IndexColumn = { name: string; type: string }
export type IndexTable = { schema: string; name: string; isView: boolean; columns: IndexColumn[] }
export type SchemaIndex = { tables: IndexTable[]; defaultSchema: string }

export type Suggestion = {
  label: string // lo que se muestra y contra lo que se filtra
  apply: string // lo que se inserta (citado si hace falta)
  raw: string // el nombre sin citar ni cualificar
  qualified: boolean // se inserta cualificado (esquema.tabla)
  kind: 'table' | 'view' | 'schema' | 'column' | 'alias'
  detail: string
  boost: number
}
/**
 * Prioridades: CodeMirror ordena por coincidencia y luego por boost (máx. 99). Lo relevante en cada contexto
 * debe ganar a las palabras clave del lenguaje (p. ej. FALSE al escribir "fa" tras un FROM).
 */
const BOOST = { member: 90, table: 80, tableOther: 70, schema: 50, column: 80, alias: 40, tableFallback: 20 }

export type Completion = { from: number; options: Suggestion[] }

// Un identificador: citado ("x", `x`, [x]) o simple.
const IDENT = String.raw`(?:"(?:[^"]|"")*"|` + '`(?:[^`]|``)*`' + String.raw`|\[(?:[^\]]|\]\])*\]|[A-Za-z_][\w$]*)`
const NAME = `${IDENT}(?:\\s*\\.\\s*${IDENT})?`

/** Palabras que NO pueden ser un alias de tabla (siguen a un nombre de tabla en la consulta). */
const NOT_ALIAS = new Set([
  'where', 'on', 'join', 'inner', 'left', 'right', 'full', 'cross', 'outer', 'natural', 'group', 'order', 'having', 'limit',
  'offset', 'union', 'intersect', 'except', 'using', 'set', 'values', 'select', 'from', 'returning', 'fetch', 'window', 'for',
  'lateral', 'as',
])

/** Estado léxico al final de text: dentro de una cadena o de un comentario no se sugiere nada. */
export function lexState(text: string): 'code' | 'string' | 'comment' {
  let i = 0
  while (i < text.length) {
    const c = text[i]
    const n = text[i + 1]
    if (c === '-' && n === '-') { // comentario de línea
      const e = text.indexOf('\n', i)
      if (e < 0) return 'comment'
      i = e + 1
    } else if (c === '/' && n === '*') { // comentario de bloque
      const e = text.indexOf('*/', i + 2)
      if (e < 0) return 'comment'
      i = e + 2
    } else if (c === "'") { // cadena: '' es una comilla escapada
      i++
      for (;;) {
        const e = text.indexOf("'", i)
        if (e < 0) return 'string'
        if (text[e + 1] === "'") { i = e + 2; continue }
        i = e + 1
        break
      }
    } else if (c === '"' || c === '`') { // identificador citado: se salta (puede quedar abierto: se está escribiendo)
      const e = text.indexOf(c, i + 1)
      if (e < 0) return 'code'
      i = e + 1
    } else i++
  }
  return 'code'
}

/** Sentencia que contiene la posición: entre el ";" anterior y el siguiente. */
function statementAround(text: string, pos: number): { stmt: string; start: number } {
  const start = text.lastIndexOf(';', pos - 1) + 1
  const end = text.indexOf(';', pos)
  return { stmt: text.slice(start, end < 0 ? text.length : end), start }
}

export type TableRef = { alias: string; table: IndexTable }

function findTable(index: SchemaIndex, parts: string[]): IndexTable | undefined {
  const low = parts.map((p) => unquoteIdent(p).toLowerCase())
  if (low.length === 2) return index.tables.find((t) => t.schema.toLowerCase() === low[0] && t.name.toLowerCase() === low[1])
  const same = index.tables.filter((t) => t.name.toLowerCase() === low[0])
  return same.find((t) => t.schema === index.defaultSchema) ?? same[0]
}

const splitName = (s: string): string[] => s.match(new RegExp(IDENT, 'g')) ?? []

/** Tablas que la sentencia declara en FROM / JOIN / UPDATE / INTO, con su alias. */
export function referencedTables(stmt: string, index: SchemaIndex): TableRef[] {
  const out: TableRef[] = []
  const add = (nameStr: string, aliasStr: string | undefined) => {
    const t = findTable(index, splitName(nameStr))
    if (!t) return
    const alias = aliasStr && !NOT_ALIAS.has(aliasStr.toLowerCase()) ? unquoteIdent(aliasStr) : ''
    out.push({ alias, table: t })
  }
  const head = new RegExp(String.raw`\b(?:FROM|JOIN|UPDATE|INTO)\s+(${NAME})(?:\s+(?:AS\s+)?(${IDENT}))?`, 'gi')
  const more = new RegExp(String.raw`\s*,\s*(${NAME})(?:\s+(?:AS\s+)?(${IDENT}))?`, 'y')
  let m: RegExpExecArray | null
  while ((m = head.exec(stmt))) {
    add(m[1], m[2])
    // FROM a x, b y, c: la lista separada por comas continúa
    more.lastIndex = head.lastIndex
    let n: RegExpExecArray | null
    while ((n = more.exec(stmt))) {
      add(n[1], n[2])
      head.lastIndex = more.lastIndex
    }
  }
  return out
}

const CLAUSES = /\b(SELECT|FROM|WHERE|GROUP|ORDER|HAVING|JOIN|ON|SET|VALUES|INTO|UPDATE|LIMIT|UNION|TABLE|USING)\b/gi

/** ¿Lo que se escribe es un nombre de tabla? (tras FROM, JOIN, INTO, UPDATE, TABLE…) */
function expectsTable(before: string): boolean {
  let last = ''
  let m: RegExpExecArray | null
  CLAUSES.lastIndex = 0
  while ((m = CLAUSES.exec(before))) last = m[1].toUpperCase()
  return last === 'FROM' || last === 'JOIN' || last === 'INTO' || last === 'UPDATE' || last === 'TABLE'
}

function tableSuggestion(driver: string, t: IndexTable, index: SchemaIndex, boost: number): Suggestion {
  const qualified = t.schema !== '' && t.schema !== index.defaultSchema
  const name = quoteIfNeeded(driver, t.name)
  const kind = t.isView ? 'vista' : 'tabla'
  return {
    // Se filtra por el nombre de la tabla (no por "esquema.tabla"): escribir "fa" debe encontrar ventas.facturas.
    label: t.name,
    raw: t.name,
    qualified,
    apply: qualified ? `${quoteIfNeeded(driver, t.schema)}.${name}` : name,
    kind: t.isView ? 'view' : 'table',
    detail: qualified ? `${t.schema} · ${kind}` : kind,
    boost,
  }
}

const columnSuggestions = (driver: string, t: IndexTable, boost: number, withTable = false): Suggestion[] =>
  t.columns.map((c) => ({
    label: c.name,
    raw: c.name,
    qualified: false,
    apply: quoteIfNeeded(driver, c.name),
    kind: 'column',
    detail: withTable ? `${c.type} · ${t.name}` : c.type,
    boost,
  }))

/**
 * Calcula las sugerencias en la posición pos de text. explicit=true cuando el usuario pide el
 * autocompletado (Ctrl+Espacio): entonces también se ofrece sin haber escrito nada.
 */
export function complete(text: string, pos: number, index: SchemaIndex | null, driver: string, explicit = false): Completion | null {
  if (!index || index.tables.length === 0) return null
  const { stmt, start } = statementAround(text, pos)
  const upto = text.slice(start, pos)
  if (lexState(upto) !== 'code') return null

  // Cadena de identificadores que termina en el cursor: [calificador.]*[parcial]
  const tail = upto.match(new RegExp(String.raw`((?:${IDENT}\s*\.\s*)*)("(?:[^"]|"")*|` + '`(?:[^`]|``)*' + String.raw`|\[(?:[^\]]|\]\])*|[A-Za-z_][\w$]*)?$`))
  if (!tail) return null
  const qualifier = tail[1] ?? ''
  const partial = tail[2] ?? ''
  const from = pos - partial.length
  if (!qualifier && !partial && !explicit) return null
  const quoteOpen = partial !== '' && /^["`[]/.test(partial)

  const refs = referencedTables(stmt, index)
  const options: Suggestion[] = []
  const seen = new Set<string>()
  const push = (s: Suggestion) => {
    const key = `${s.kind}|${s.label}|${s.detail}`
    if (!seen.has(key)) { seen.add(key); options.push(s) }
  }

  if (qualifier) {
    const parts = splitName(qualifier)
    if (parts.length === 1) {
      const q = unquoteIdent(parts[0]).toLowerCase()
      const ref = refs.find((r) => r.alias.toLowerCase() === q) ?? refs.find((r) => r.table.name.toLowerCase() === q)
      if (ref) columnSuggestions(driver, ref.table, BOOST.member).forEach(push)
      else {
        const asTable = findTable(index, parts)
        if (asTable) columnSuggestions(driver, asTable, BOOST.member).forEach(push) // "tabla." sin haberla puesto en FROM
        // "esquema." → sus tablas
        index.tables
          .filter((t) => t.schema.toLowerCase() === q)
          .forEach((t) => push({ label: t.name, raw: t.name, qualified: false, apply: quoteIfNeeded(driver, t.name), kind: t.isView ? 'view' : 'table', detail: t.isView ? 'vista' : 'tabla', boost: BOOST.member }))
      }
    } else if (parts.length === 2) {
      const t = findTable(index, parts)
      if (t) columnSuggestions(driver, t, BOOST.member).forEach(push)
    }
  } else if (expectsTable(upto.slice(0, from - start))) {
    // Tras FROM / JOIN / INTO / UPDATE: tablas y vistas (las no cualificadas primero) y los nombres de esquema.
    index.tables.forEach((t) => push(tableSuggestion(driver, t, index, t.schema === '' || t.schema === index.defaultSchema ? BOOST.table : BOOST.tableOther)))
    new Set(index.tables.map((t) => t.schema).filter((s) => s !== '' && s !== index.defaultSchema)).forEach((s) =>
      push({ label: s, raw: s, qualified: false, apply: quoteIfNeeded(driver, s), kind: 'schema', detail: 'esquema', boost: BOOST.schema }))
  } else {
    // En una expresión: columnas de las tablas de la consulta, y los alias/tablas para escribir "alias.columna".
    const many = refs.length > 1
    refs.forEach((r) => columnSuggestions(driver, r.table, BOOST.column, many).forEach(push))
    refs.forEach((r) => push({ label: r.alias || r.table.name, raw: r.alias || r.table.name, qualified: false, apply: r.alias ? quoteIfNeeded(driver, r.alias) : quoteIfNeeded(driver, r.table.name), kind: 'alias', detail: r.alias ? `alias de ${r.table.name}` : 'tabla', boost: BOOST.alias }))
    if (refs.length === 0) index.tables.forEach((t) => push(tableSuggestion(driver, t, index, BOOST.tableFallback)))
  }

  if (options.length === 0) return null

  if (quoteOpen) {
    // El usuario ya abrió comillas: se completa lo que va dentro y se cierran. Las tablas cualificadas
    // (esquema.tabla) no encajan dentro de unas comillas, así que se omiten.
    const closer = { '"': '"', '`': '`', '[': ']' }[partial[0] as '"' | '`' | '['] ?? '"'
    const esc = (n: string) => (closer === ']' ? n.replaceAll(']', ']]') : n.replaceAll(closer, closer + closer))
    return {
      from: from + 1,
      options: options.filter((o) => !o.qualified).map((o) => ({ ...o, apply: esc(o.raw) + closer })),
    }
  }
  return { from, options }
}
