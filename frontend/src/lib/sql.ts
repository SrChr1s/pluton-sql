// Utilidades SQL puras (sin dependencias de Svelte ni de Wails), compartidas por la interfaz y los tests.

/** Palabras que, usadas como nombre, hay que citar en cualquier motor. */
const RESERVED = new Set([
  'select', 'from', 'where', 'order', 'group', 'by', 'table', 'user', 'key', 'index', 'primary', 'references', 'check',
  'default', 'limit', 'offset', 'join', 'on', 'as', 'and', 'or', 'not', 'null', 'case', 'when', 'then', 'end', 'union',
  'values', 'set', 'update', 'delete', 'insert', 'into', 'create', 'drop', 'alter', 'column', 'database', 'schema', 'view',
  'desc', 'asc', 'having', 'distinct', 'all', 'between', 'like', 'in', 'is', 'exists', 'grant', 'to', 'role', 'constraint',
  'foreign', 'unique', 'trigger', 'transaction', 'commit', 'rollback', 'left', 'right', 'inner', 'outer', 'cross', 'using',
])

/** Comillas de identificador según el motor. */
export function quoteIdent(driver: string, name: string): string {
  if (driver === 'mysql') return '`' + name.replaceAll('`', '``') + '`'
  if (driver === 'mssql') return '[' + name.replaceAll(']', ']]') + ']'
  return '"' + name.replaceAll('"', '""') + '"'
}

/** ¿Hay que citar este nombre para que el motor lo entienda tal cual? */
export function needsQuote(driver: string, name: string): boolean {
  if (RESERVED.has(name.toLowerCase())) return true
  // PostgreSQL pliega a minúsculas lo que no va entre comillas: un nombre con mayúsculas solo se alcanza citado.
  if (driver === 'postgres') return !/^[a-z_][a-z0-9_$]*$/.test(name)
  return !/^[A-Za-z_][A-Za-z0-9_$]*$/.test(name)
}

/** Cita el nombre solo si hace falta. */
export function quoteIfNeeded(driver: string, name: string): string {
  return needsQuote(driver, name) ? quoteIdent(driver, name) : name
}

/** Quita las comillas de un identificador citado ("x", `x`, [x]) y des-escapa las comillas dobles internas. */
export function unquoteIdent(s: string): string {
  if (s.length >= 2) {
    const a = s[0]
    const b = s[s.length - 1]
    if (a === '"' && b === '"') return s.slice(1, -1).replaceAll('""', '"')
    if (a === '`' && b === '`') return s.slice(1, -1).replaceAll('``', '`')
    if (a === '[' && b === ']') return s.slice(1, -1).replaceAll(']]', ']')
  }
  return s
}
