/** Interpreta el texto de una celda como JSON; solo objetos y arrays (un número o "texto" sueltos no merecen visor). */
export function parseJson(text: string | null | undefined): unknown | undefined {
  if (!text) return undefined
  const s = text.trimStart()
  if (s[0] !== '{' && s[0] !== '[') return undefined
  try {
    const v = JSON.parse(text)
    return v !== null && typeof v === 'object' ? v : undefined
  } catch {
    return undefined
  }
}

/** Ruta tipo JSONPath de un hijo: `$.a.b[0]`, con `["clave rara"]` cuando no es un identificador. */
export function childPath(parent: string, key: string | number): string {
  if (typeof key === 'number') return `${parent}[${key}]`
  return /^[A-Za-z_$][\w$]*$/.test(key) ? `${parent}.${key}` : `${parent}[${JSON.stringify(key)}]`
}

/** Una fila como objeto JSON {columna: valor}; el texto que es JSON se incrusta ya interpretado. */
export function rowToJson(columns: string[], values: unknown[]): string {
  const o: Record<string, unknown> = {}
  columns.forEach((c, i) => {
    const v = values[i]
    o[c] = typeof v === 'string' ? (parseJson(v) ?? v) : (v ?? null)
  })
  return JSON.stringify(o)
}
